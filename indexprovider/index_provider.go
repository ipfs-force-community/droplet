package indexprovider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipni/go-libipni/metadata"
	provider "github.com/ipni/index-provider"
	"github.com/ipni/index-provider/engine"
	"github.com/ipni/index-provider/engine/xproviders"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/filecoin-project/go-fil-markets/stores"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	types "github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/ipfs-force-community/droplet/v2/config"
	"github.com/ipfs-force-community/droplet/v2/models/repo"
)

var log = logging.Logger("index-provider-wrapper")

type Wrapper struct {
	enabled bool

	h             host.Host
	dagStore      stores.DAGStoreWrapper
	full          v1.FullNode
	cfg           *config.ProviderConfig
	dealsDB       repo.StorageDealRepo
	directDealsDB repo.DirectDealRepo
	prov          provider.Interface

	meshCreator MeshCreator
	// bitswapEnabled records whether to announce bitswap as an available
	// protocol to the network indexer
	bitswapEnabled bool
	httpEnabled    bool
	stop           context.CancelFunc
}

func NewWrapper(h host.Host,
	cfg *config.ProviderConfig,
	full v1.FullNode,
	r repo.Repo,
	dagStore stores.DAGStoreWrapper,
	prov provider.Interface,
) (*Wrapper, error) {
	_, isDisabled := prov.(*DisabledIndexProvider)

	// todo: support bitswap
	// bitswap is enabled if there is a bitswap peer id
	bitswapEnabled := false
	// http is considered enabled if there is an http retrieval multiaddr set
	httpEnabled := cfg.HTTPRetrievalMultiaddr != ""

	// setup bitswap extended provider if there is a public multi addr for bitswap
	w := &Wrapper{
		h:              h,
		dealsDB:        r.StorageDealRepo(),
		directDealsDB:  r.DirectDealRepo(),
		prov:           prov,
		meshCreator:    NewMeshCreator(full, h),
		cfg:            cfg,
		enabled:        !isDisabled,
		bitswapEnabled: bitswapEnabled,
		httpEnabled:    httpEnabled,
		full:           full,
		dagStore:       dagStore,
	}

	return w, nil
}

func (w *Wrapper) Start(ctx context.Context) {
	w.prov.RegisterMultihashLister(w.MultihashLister)

	runCtx, runCancel := context.WithCancel(ctx)
	w.stop = runCancel

	// Announce all deals on startup in case of a config change
	go func() {
		err := w.AnnounceExtendedProviders(runCtx)
		if err != nil {
			log.Warnf("announcing extended providers: %s", err)
		}
	}()

	log.Info("starting index provider")
}

func (w *Wrapper) Stop() {
	w.stop()
}

func (w *Wrapper) Enabled() bool {
	return w.enabled
}

// AnnounceExtendedProviders announces changes to Market configuration in the context of retrieval
// methods.
//
// The advertisement published by this function covers 2 protocols:
//
// Bitswap:
//
//  1. bitswap is completely disabled: in which case an advertisement is
//     published with http(or empty if http is disabled) extended providers
//     that should wipe previous support on indexer side.
//
//  2. bitswap is enabled with public addresses: in which case publish an
//     advertisement with extended providers records corresponding to the
//     public addresses. Note, according the IPNI spec, the host ID will
//     also be added to the extended providers for signing reasons with empty
//     metadata making a total of 2 extended provider records.
//
//  3. bitswap with droplet address: in which case public an advertisement
//     with one extended provider record that just adds bitswap metadata.
//
// HTTP:
//
//  1. http is completely disabled: in which case an advertisement is
//     published with bitswap(or empty if bitswap is disabled) extended providers
//     that should wipe previous support on indexer side
//
//  2. http is enabled: in which case an advertisement is published with
//     bitswap and http(or only http if bitswap is disabled) extended providers
//     that should wipe previous support on indexer side
//
//     Note that in any case one advertisement is published by droplet on startup
//     to reflect on extended provider configuration, even if the config remains the
//     same. Future work should detect config change and only publish ads when
//     config changes.
func (w *Wrapper) AnnounceExtendedProviders(ctx context.Context) error {
	if !w.enabled {
		return errors.New("cannot announce all deals: index provider is disabled")
	}
	// for now, only generate an indexer provider announcement if bitswap announcements
	// are enabled -- all other graphsync announcements are context ID specific

	// build the extended providers announcement
	key := w.h.Peerstore().PrivKey(w.h.ID())
	adBuilder := xproviders.NewAdBuilder(w.h.ID(), key, w.h.Addrs())

	err := w.appendExtendedProviders(ctx, adBuilder, key)
	if err != nil {
		return err
	}

	last, _, err := w.prov.GetLatestAdv(ctx)
	if err != nil {
		return err
	}
	adBuilder.WithLastAdID(last)
	ad, err := adBuilder.BuildAndSign()
	if err != nil {
		return err
	}

	// make sure we're connected to the mesh so that the message will go through
	// pubsub and reach the indexer
	err = w.meshCreator.Connect(ctx)
	if err != nil {
		log.Warnf("could not connect to pubsub mesh before announcing extended provider: %v", err)
	}

	// publish the extended providers announcement
	adCid, err := w.prov.Publish(ctx, *ad)
	if err != nil {
		return err
	}

	log.Infof("announced endpoint to indexer with advertisement cid %s", adCid)

	return nil
}

func (w *Wrapper) appendExtendedProviders(_ context.Context, adBuilder *xproviders.AdBuilder, key crypto.PrivKey) error {
	// if !w.bitswapEnabled {
	// 	// If bitswap is completely disabled, publish an advertisement with empty extended providers
	// 	// which should override previously published extended providers associated to w.h.ID().
	// 	log.Info("bitswap is not enabled - announcing bitswap disabled to Indexer")
	// } else {
	// if we're exposing bitswap publicly, we announce bitswap as an extended provider. If we're not
	// we announce it as metadata on the main provider

	// marshal bitswap metadata
	// meta := metadata.Default.New(metadata.Bitswap{})
	// mbytes, err := meta.MarshalBinary()
	// if err != nil {
	// 	return err
	// }
	// var ep xproviders.Info
	// if len(w.cfg.Retrievals.Bitswap.BitswapPublicAddresses) > 0 {
	// 	if w.cfg.Retrievals.Bitswap.BitswapPrivKeyFile == "" {
	// 		return fmt.Errorf("missing required configuration key BitswapPrivKeyFile: " +
	// 			"droplet is configured with BitswapPublicAddresses but the BitswapPrivKeyFile configuration key is empty")
	// 	}

	// 	// we need the private key for bitswaps peerID in order to announce publicly
	// 	keyFile, err := os.ReadFile(w.cfg.Retrievals.Bitswap.BitswapPrivKeyFile)
	// 	if err != nil {
	// 		return fmt.Errorf("opening BitswapPrivKeyFile %s: %w", w.cfg.Retrievals.Bitswap.BitswapPrivKeyFile, err)
	// 	}
	// 	privKey, err := crypto.UnmarshalPrivateKey(keyFile)
	// 	if err != nil {
	// 		return fmt.Errorf("unmarshalling BitswapPrivKeyFile %s: %w", w.cfg.Retrievals.Bitswap.BitswapPrivKeyFile, err)
	// 	}
	// 	// setup an extended provider record, containing the booster-bitswap multi addr,
	// 	// peer ID, private key for signing, and metadata
	// 	ep = xproviders.Info{
	// 		ID:       w.cfg.Retrievals.Bitswap.BitswapPeerID,
	// 		Addrs:    w.cfg.Retrievals.Bitswap.BitswapPublicAddresses,
	// 		Priv:     privKey,
	// 		Metadata: mbytes,
	// 	}
	// 	log.Infof("bitswap is enabled and endpoint is public - "+
	// 		"announcing bitswap endpoint to indexer as extended provider: %s %s",
	// 		ep.ID, ep.Addrs)
	// } else {
	// 	log.Infof("bitswap is enabled with boostd as proxy - "+
	// 		"announcing boostd as endpoint for bitswap to indexer: %s %s",
	// 		w.h.ID(), w.h.Addrs())

	// 	addrs := make([]string, 0, len(w.h.Addrs()))
	// 	for _, addr := range w.h.Addrs() {
	// 		addrs = append(addrs, addr.String())
	// 	}

	// 	ep = xproviders.Info{
	// 		ID:       w.h.ID().String(),
	// 		Addrs:    addrs,
	// 		Priv:     key,
	// 		Metadata: mbytes,
	// 	}
	// }
	// adBuilder.WithExtendedProviders(ep)
	// }

	if !w.httpEnabled {
		log.Info("ProviderConfig.HTTPRetrievalMultiaddr is not set - announcing http disabled to Indexer")
	} else {
		// marshal http metadata
		meta := metadata.Default.New(metadata.IpfsGatewayHttp{})
		mbytes, err := meta.MarshalBinary()
		if err != nil {
			return err
		}
		var ep = xproviders.Info{
			ID:       w.h.ID().String(),
			Addrs:    []string{w.cfg.HTTPRetrievalMultiaddr},
			Metadata: mbytes,
			Priv:     key,
		}

		log.Infof("announcing http endpoint to indexer as extended provider: %s", ep.Addrs)

		adBuilder.WithExtendedProviders(ep)
	}

	return nil
}

// ErrStringSkipAdIngest - While ingesting cids for each piece, if there is an error the indexer
// checks if the error contains the string "content not found":
// - if so, the indexer skips the piece and continues ingestion
// - if not, the indexer pauses ingestion
var ErrStringSkipAdIngest = "content not found"

func skipError(err error) error {
	return fmt.Errorf("%s: %s: %w", ErrStringSkipAdIngest, err.Error(), ipld.ErrNotExists{})
}

func (w *Wrapper) IndexerAnnounceLatest(ctx context.Context) (cid.Cid, error) {
	e, ok := w.prov.(*engine.Engine)
	if !ok {
		return cid.Undef, fmt.Errorf("index provider is disabled")
	}
	return e.PublishLatest(ctx)
}

func (w *Wrapper) IndexerAnnounceLatestHttp(ctx context.Context, announceUrls []string) (cid.Cid, error) {
	e, ok := w.prov.(*engine.Engine)
	if !ok {
		return cid.Undef, fmt.Errorf("index provider is disabled")
	}

	if len(announceUrls) == 0 {
		announceUrls = w.cfg.IndexProvider.Announce.DirectAnnounceURLs
	}

	urls := make([]*url.URL, 0, len(announceUrls))
	for _, us := range announceUrls {
		u, err := url.Parse(us)
		if err != nil {
			return cid.Undef, fmt.Errorf("parsing url %s: %w", us, err)
		}
		urls = append(urls, u)
	}
	return e.PublishLatestHTTP(ctx, urls...)
}

func (w *Wrapper) MultihashLister(ctx context.Context, prov peer.ID, contextID []byte) (provider.MultihashIterator, error) {
	provideF := func(identifier string, isDD bool, pieceCid cid.Cid) (provider.MultihashIterator, error) {
		idName := "propCid"
		if isDD {
			idName = "UUID"
		}
		llog := log.With(idName, identifier, "piece", pieceCid)
		ii, err := w.dagStore.GetIterableIndexForPiece(pieceCid)
		if err != nil {
			e := fmt.Errorf("failed to get iterable index: %w", err)
			if strings.Contains(err.Error(), "file does not exist") ||
				strings.Contains(err.Error(), mongo.ErrNoDocuments.Error()) {
				// If it's a not found error, skip over this piece and continue ingesting
				llog.Infow("skipping ingestion: piece not found", "err", e)
				return nil, skipError(e)
			}

			// Some other error, pause ingestion
			llog.Infow("pausing ingestion: error getting piece", "err", e)
			return nil, e
		}

		// Check if there are any records in the iterator.
		hasRecords := ii.ForEach(func(_ multihash.Multihash, _ uint64) error {
			return fmt.Errorf("has at least one record")
		})
		if hasRecords == nil {
			// If there are no records, it's effectively the same as a not
			// found error. Skip over this piece and continue ingesting.
			e := fmt.Errorf("no records found for piece %s", pieceCid)
			llog.Infow("skipping ingestion: piece has no records", "err", e)
			return nil, skipError(e)
		}

		mhi, err := provider.CarMultihashIterator(ii)
		if err != nil {
			// Bad index, skip over this piece and continue ingesting
			err = fmt.Errorf("failed to get mhiterator: %w", err)
			llog.Infow("skipping ingestion", "err", err)
			return nil, skipError(err)
		}

		llog.Debugw("returning piece iterator", "err", err)
		return mhi, nil
	}

	// Try to cast the context to a proposal CID for droplet deals and legacy deals
	proposalCid, err := cid.Cast(contextID)
	if err == nil {
		// Look up deal by proposal cid in the droplet database.
		// If we can't find it there check legacy markets DB.
		pds, dealErr := w.dealsDB.GetDeal(ctx, proposalCid)
		if dealErr == nil {
			// Found the deal, get an iterator over the piece
			pieceCid := pds.ClientDealProposal.Proposal.PieceCID
			return provideF(proposalCid.String(), false, pieceCid)
		}

		// Check if it's a "not found" error
		if !errors.Is(dealErr, repo.ErrNotFound) {
			// It's not a "not found" error: there was a problem accessing the
			// database. Pause ingestion until the user can fix the DB.
			e := fmt.Errorf("getting deal with proposal cid %s from droplet database: %w", proposalCid, dealErr)
			log.Infow("pausing ingestion", "proposalCid", proposalCid, "err", e)
			return nil, e
		}

		// The deal was not found in the droplet or legacy database.
		// Skip this deal and continue ingestion.
		err = fmt.Errorf("deal with proposal cid %s not found", proposalCid)
		log.Infow("skipping ingestion", "proposalCid", proposalCid, "err", err)
		return nil, skipError(err)
	}

	dealUUID, err := uuid.FromBytes(contextID)
	if err == nil {
		// Look up deal by dealUUID in the direct deals database
		entry, dderr := w.directDealsDB.GetDeal(ctx, dealUUID)
		if dderr == nil {
			// Found the deal, get an iterator over the piece
			return provideF(dealUUID.String(), true, entry.PieceCID)
		}

		// Check if it's a "not found" error
		if !errors.Is(dderr, repo.ErrNotFound) {
			// It's not a "not found" error: there was a problem accessing the
			// database. Pause ingestion until the user can fix the DB.
			e := fmt.Errorf("getting deal with UUID %s from direct deal database: %w", dealUUID, dderr)
			log.Infow("pausing ingestion", "deal UUID", dealUUID, "err", e)
			return nil, e
		}

		// The deal was not found in the droplet, legacy or direct deal database.
		// Skip this deal and continue ingestion.
		err = fmt.Errorf("deal with UUID %s not found", dealUUID)
		log.Infow("skipping ingestion", "deal UUID", dealUUID, "err", err)
		return nil, skipError(err)
	}

	// Bad contextID or UUID skip over this piece and continue ingesting
	err = fmt.Errorf("failed to cast context ID to a cid and UUID")
	log.Infow("skipping ingestion", "context ID", string(contextID), "err", err)
	return nil, skipError(err)
}

func (w *Wrapper) AnnounceDeal(ctx context.Context, deal *types.MinerDeal) (cid.Cid, error) {
	// Filter out deals that should not be announced
	// if !deal.AnnounceToIPNI {
	// 	return cid.Undef, nil
	// }

	md := metadata.GraphsyncFilecoinV1{
		PieceCID:      deal.ClientDealProposal.Proposal.PieceCID,
		FastRetrieval: deal.FastRetrieval,
		VerifiedDeal:  deal.ClientDealProposal.Proposal.VerifiedDeal,
	}
	c, err := w.AnnounceDealMetadata(ctx, md, deal.ProposalCid.Bytes())
	if err != nil {
		return c, err
	}
	label, _ := deal.Proposal.Label.ToString()
	log.Infof("announced deal to index provider success: %s, %s, ad cid: %v", deal.ProposalCid, label, c)

	return c, nil
}

func (w *Wrapper) AnnounceDealMetadata(ctx context.Context, md metadata.GraphsyncFilecoinV1, contextID []byte) (cid.Cid, error) {
	if !w.enabled {
		return cid.Undef, errors.New("cannot announce deal: index provider is disabled")
	}

	// Ensure we have a connection with the full node host so that the index provider gossip sub announcements make their
	// way to the filecoin bootstrapper network
	if err := w.meshCreator.Connect(ctx); err != nil {
		log.Errorw("failed to connect node to full daemon node", "err", err)
	}

	// Announce deal to network Indexer
	fm := metadata.Default.New(&md)
	annCid, err := w.prov.NotifyPut(ctx, nil, contextID, fm)
	if err != nil {
		// Check if the error is because the deal was already advertised
		// (we can safely ignore this error)
		if !errors.Is(err, provider.ErrAlreadyAdvertised) {
			return cid.Undef, fmt.Errorf("failed to announce deal to index provider: %w", err)
		}
	}
	return annCid, nil
}

func (w *Wrapper) AnnounceDealRemoved(ctx context.Context, contextID []byte) (cid.Cid, error) {
	if !w.enabled {
		return cid.Undef, errors.New("cannot announce deal removal: index provider is disabled")
	}

	// Ensure we have a connection with the full node host so that the index provider gossip sub announcements make their
	// way to the filecoin bootstrapper network
	if err := w.meshCreator.Connect(ctx); err != nil {
		log.Errorw("failed to connect node to full daemon node", "err", err)
	}

	// Announce deal removal to network Indexer
	annCid, err := w.prov.NotifyRemove(ctx, "", contextID)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to announce deal removal to index provider: %w", err)
	}
	return annCid, err
}

func (w *Wrapper) AnnounceDirectDeal(ctx context.Context, entry *types.DirectDeal) (cid.Cid, error) {
	// Filter out deals that should not be announced
	// if !entry.AnnounceToIPNI {
	// 	return cid.Undef, nil
	// }

	contextID, err := entry.ID.MarshalBinary()
	if err != nil {
		return cid.Undef, fmt.Errorf("marshalling the deal UUID: %w", err)
	}

	md := metadata.GraphsyncFilecoinV1{
		PieceCID:      entry.PieceCID,
		FastRetrieval: true,
		VerifiedDeal:  true,
	}
	c, err := w.AnnounceDealMetadata(ctx, md, contextID)
	if err != nil {
		return c, err
	}
	log.Infof("announced direct deal to index provider success: %s, ad cid: %v", entry.ID, c)
	return c, nil
}
