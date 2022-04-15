package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	"github.com/jpillora/backoff"
	"github.com/libp2p/go-libp2p-core/host"
	p2phttp "github.com/libp2p/go-libp2p-http"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus-market/types"
)

var log = logger.Logger("http-transport")

const (
	// 1 Mib
	readBufferSize = 1048576

	minBackOff           = 1 * time.Minute
	maxBackOff           = 1 * time.Hour
	factor               = 2
	maxReconnectAttempts = 8

	libp2pScheme = "libp2p"
)

var _ Transport = (*HttpTransport)(nil)

type Option func(*HttpTransport)

func BackOffRetryOpt(minBackoff, maxBackoff time.Duration, factor, maxReconnectAttempts float64) Option {
	return func(h *HttpTransport) {
		h.minBackOffWait = minBackoff
		h.maxBackoffWait = maxBackoff
		h.backOffFactor = factor
		h.maxReconnectAttempts = maxReconnectAttempts
	}
}

func ReadBufferSize(size uint64) Option {
	return func(h *HttpTransport) {
		h.readBufferSize = size
	}
}

func MaxConcurrent(maxConcurrent int) Option {
	return func(h *HttpTransport) {
		h.maxConcurrent = maxConcurrent
	}
}

type HttpTransport struct {
	libp2pHost   host.Host
	libp2pClient *http.Client

	minBackOffWait       time.Duration
	maxBackoffWait       time.Duration
	backOffFactor        float64
	maxReconnectAttempts float64
	readBufferSize       uint64
	maxConcurrent        int
}

func New(host host.Host, opts ...Option) *HttpTransport {
	ht := &HttpTransport{
		libp2pHost:           host,
		minBackOffWait:       minBackOff,
		maxBackoffWait:       maxBackOff,
		backOffFactor:        factor,
		maxReconnectAttempts: maxReconnectAttempts,
	}
	for _, o := range opts {
		o(ht)
	}

	// init a libp2p-http client
	tr := &http.Transport{}
	p2ptr := p2phttp.NewTransport(host, p2phttp.ProtocolOption(types.DataTransferProtocol))
	tr.RegisterProtocol("libp2p", p2ptr)
	ht.libp2pClient = &http.Client{Transport: tr}

	return ht
}

func (h *HttpTransport) Execute(ctx context.Context, info *types.TransportInfo) (th Handler, err error) {
	tLog := log.With("proposal cid", info.ProposalCID)

	deadline, _ := ctx.Deadline()
	tLog.Infof("execute transfer, deal size %v, output file %v, time before context deadline %v",
		info.Transfer.Size, info.OutputFile, time.Until(deadline).String())

	// de-serialize transport opaque token
	tInfo := &types.HttpRequest{}
	if err := json.Unmarshal(info.Transfer.Params, tInfo); err != nil {
		return nil, xerrors.Errorf("failed to de-serialize transport info bytes, bytes:%s, err:%w", string(info.Transfer.Params), err)
	}

	if len(tInfo.URL) == 0 {
		return nil, xerrors.New("deal url is empty")
	}

	// parse request URL
	u, err := parseUrl(tInfo.URL)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse request url: %w", err)
	}
	tInfo.URL = u.url

	// check that the outputFile exists
	fi, err := os.Stat(info.OutputFile)
	if err != nil {
		return nil, xerrors.Errorf("output file state error: %w", err)
	}

	// do we have more bytes than required already ?
	fileSize := fi.Size()
	if fileSize > int64(info.Transfer.Size) {
		return nil, xerrors.Errorf("deal size=%d but file size=%d", info.Transfer.Size, fileSize)
	}
	tLog.Infof("existing file size %d, deal size %d", fileSize, info.Transfer.Size)

	tctx, t := h.newTransfer(ctx, tInfo, info, u, fileSize)

	// is the transfer already complete ? we check this by comparing the number of bytes
	// in the output file with the deal size.
	if fileSize == int64(info.Transfer.Size) {
		defer close(t.eventCh)
		defer t.cancel()

		if err := t.emitEvent(tctx, types.TransportEvent{
			NBytesReceived: fileSize,
		}, info.ProposalCID); err != nil {
			return nil, xerrors.Errorf("failed to publish transfer completion event, proposal cid: %s, err: %w", t.info.ProposalCID, err)
		}

		tLog.Infof("file size is already equal to deal size, returning")
		return t, nil
	}

	t.start(tctx)

	return t, nil
}

func (h *HttpTransport) newTransfer(ctx context.Context,
	tInfo *types.HttpRequest,
	info *types.TransportInfo,
	u *transportUrl,
	fileSize int64) (context.Context, *transfer) {
	// construct the transfer instance that will act as the transfer handler
	ctx, cancel := context.WithCancel(ctx)
	t := &transfer{
		cancel:         cancel,
		tInfo:          tInfo,
		info:           info,
		eventCh:        make(chan types.TransportEvent, 256),
		nBytesReceived: fileSize,
		backoff: &backoff.Backoff{
			Min:    h.minBackOffWait,
			Max:    h.maxBackoffWait,
			Factor: h.backOffFactor,
			Jitter: true,
		},
		maxReconnectAttempts: h.maxReconnectAttempts,
	}

	// If this is a libp2p URL
	if u.scheme == libp2pScheme {
		// Use the libp2p client
		t.client = h.libp2pClient
		// Add the peer's address to the peerstore so we can dial it
		addrTtl := time.Hour
		if deadline, ok := ctx.Deadline(); ok {
			addrTtl = time.Until(deadline)
		}
		h.libp2pHost.Peerstore().AddAddr(u.peerID, u.multiaddr, addrTtl)
		log.Infof("libp2p-http url %v, peer id %v, multiaddr %v, proposal cid %v", tInfo.URL, u.peerID, u.multiaddr, info.ProposalCID)
	} else {
		t.client = http.DefaultClient
		log.Infof("http url %v, proposal cid %v", tInfo.URL, info.ProposalCID)
	}

	return ctx, t
}

type transfer struct {
	closeOnce sync.Once
	cancel    context.CancelFunc

	eventCh chan types.TransportEvent

	tInfo *types.HttpRequest
	info  *types.TransportInfo
	wg    sync.WaitGroup

	nBytesReceived int64

	backoff              *backoff.Backoff
	maxReconnectAttempts float64

	client *http.Client
}

func (t *transfer) start(ctx context.Context) {
	// start executing the transfer
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer close(t.eventCh)
		defer t.cancel()

		if err := t.execute(ctx); err != nil {
			if err := t.emitEvent(ctx, types.TransportEvent{
				Error: err,
			}, t.info.ProposalCID); err != nil {
				log.Errorf("failed to publish transport, proposal cid %s, error: %v", t.info.ProposalCID, err)
			}
		}
	}()

	log.Infof("started async http transfer %v", t.info.ProposalCID)
}

func (t *transfer) emitEvent(ctx context.Context, evt types.TransportEvent, proposalCID cid.Cid) error {
	select {
	case t.eventCh <- evt:
		return nil
	default:
		return fmt.Errorf("dropping event %+v as channel is full for proposal cid %s", evt, proposalCID)
	}
}

func (t *transfer) execute(ctx context.Context) error {
	tLog := log.With("proposal cid", t.info.ProposalCID)
	for {
		// construct request
		req, err := http.NewRequest("GET", t.tInfo.URL, nil)
		if err != nil {
			return xerrors.Errorf("failed to create http req: %w", err)
		}

		// get the number of bytes already received (the size of the output file)
		st, err := os.Stat(t.info.OutputFile)
		if err != nil {
			return xerrors.Errorf("failed to stat output file: %w", err)
		}
		t.nBytesReceived = st.Size()

		// add request headers
		for name, val := range t.tInfo.Headers {
			req.Header.Set(name, val)
		}

		// add range req to start reading from the last byte we have in the output file
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", t.nBytesReceived))
		// init the request with the transfer context
		req = req.WithContext(ctx)
		// open output file in append-only mode for writing
		of, err := os.OpenFile(t.info.OutputFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return xerrors.Errorf("failed to open output file: %w", err)
		}
		defer of.Close() // nolint

		// start the http transfer
		remaining := int64(t.info.Transfer.Size) - t.nBytesReceived
		reqErr := t.doHttp(ctx, req, of, remaining)
		if reqErr != nil {
			tLog.Infof("one http req done, http code %v , error %v", reqErr.code, reqErr.Error())
		} else {
			tLog.Infof("http req finished with no error")
		}

		if reqErr == nil {
			tLog.Infof("http req done without any errors")
			// if there's no error, transfer was successful
			break
		}
		_ = of.Close()

		// check if the error is a 4xx error, meaning there is a problem with
		// the request (eg 401 Unauthorized)
		if reqErr.code/100 == 4 {
			msg := fmt.Sprintf("terminating http req: received %d response from server", reqErr.code)
			tLog.Errorf("%s, error: %s", msg, reqErr.Error())
			return reqErr.error
		}

		// do not resume transfer if context has been cancelled or if the context deadline has exceeded
		err = reqErr.error
		if xerrors.Is(err, context.Canceled) || xerrors.Is(err, context.DeadlineExceeded) {
			tLog.Errorf("terminating http req as context cancelled or deadline exceeded %v", err)
			return xerrors.Errorf("transfer context err: %w", err)
		}

		// backoff-retry transfer if max number of attempts haven't been exhausted
		nAttempts := t.backoff.Attempt() + 1
		if nAttempts >= t.maxReconnectAttempts {
			tLog.Errorf("terminating http req as exhausted max attempts, err %v, maxAttempts %v", err.Error(), t.maxReconnectAttempts)
			return xerrors.Errorf("could not finish transfer even after %.0f attempts, lastErr: %w", t.maxReconnectAttempts, err)
		}
		duration := t.backoff.Duration()
		bt := time.NewTimer(duration)
		tLog.Infof("retrying http req after waiting wait time %v, nAttempts %v", duration.String(), nAttempts)
		defer bt.Stop()
		select {
		case <-bt.C:
		case <-ctx.Done():
			tLog.Errorf("did not proceed with retry as context cancelled %v", ctx.Err())
			return xerrors.Errorf("transfer context err after %.0f attempts to finish transfer, lastErr=%s, contextErr=%w", t.backoff.Attempt(), err, ctx.Err())
		}
	}

	// --- http request finished successfully. see if we got the number of bytes we expected.

	// if the number of bytes we've received is not the same as the deal size, we have a failure.
	if t.nBytesReceived != int64(t.info.Transfer.Size) {
		return xerrors.Errorf("mismatch in dealSize vs received bytes, dealSize=%d, received=%d", t.info.Transfer.Size, t.nBytesReceived)
	}
	// if the file size is not equal to the number of bytes received, something has gone wrong
	st, err := os.Stat(t.info.OutputFile)
	if err != nil {
		return xerrors.Errorf("failed to stat output file: %w", err)
	}
	if t.nBytesReceived != st.Size() {
		return xerrors.Errorf("mismatch in output file size vs received bytes, fileSize=%d, receivedBytes=%d", st.Size(), t.nBytesReceived)
	}

	tLog.Infof("http req finished successfully, nBytesReceived %v, file size %v", t.nBytesReceived, st.Size())

	return nil
}

func (t *transfer) doHttp(ctx context.Context, req *http.Request, dst io.Writer, toRead int64) *httpError {
	tLog := log.With("proposal cid", t.info.ProposalCID)
	tLog.Infof("sending http req, received %v, remaining %v, range-rq %v", t.nBytesReceived, toRead, req.Header.Get("Range"))

	// send http request and validate response
	resp, err := t.client.Do(req)
	if err != nil {
		return &httpError{error: fmt.Errorf("failed to send  http req: %w", err)}
	}
	// we should either get back a 200 or a 206 -> anything else means something has gone wrong and we return an error.
	defer resp.Body.Close() // nolint
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return &httpError{
			error: xerrors.Errorf("http req failed: code: %d, status: %s", resp.StatusCode, resp.Status),
			code:  resp.StatusCode,
		}
	}

	//  start reading the response stream `readBufferSize` at a time using a limit reader so we only read as many bytes as we need to.
	buf := make([]byte, readBufferSize)
	limitR := io.LimitReader(resp.Body, toRead)
	for {
		if ctx.Err() != nil {
			tLog.Errorf("not reading http response anymore %v", ctx.Err())
			return &httpError{error: ctx.Err()}
		}
		nr, readErr := limitR.Read(buf)

		// if we read more than zero bytes, write whatever read.
		if nr > 0 {
			nw, writeErr := dst.Write(buf[0:nr])

			// if the number of read and written bytes don't match -> something has gone wrong, abort the http req.
			if nw < 0 || nr != nw {
				if writeErr != nil {
					return &httpError{error: fmt.Errorf("failed to write to output file: %w", writeErr)}
				}
				return &httpError{error: fmt.Errorf("read-write mismatch writing to the output file, read=%d, written=%d", nr, nw)}
			}

			t.nBytesReceived = t.nBytesReceived + int64(nw)

			// emit event updating the number of bytes received
			if err := t.emitEvent(ctx, types.TransportEvent{
				NBytesReceived: t.nBytesReceived,
			}, t.info.ProposalCID); err != nil {
				tLog.Errorf("failed to publish transport event %v", err)
			}
		}
		// the http stream we're reading from has sent us an EOF, nothing to do here.
		if readErr == io.EOF {
			tLog.Infof("http server sent EOF, received %d, deal-size %d", t.nBytesReceived, t.info.Transfer.Size)
			return nil
		}
		if readErr != nil {
			return &httpError{error: fmt.Errorf("error reading from http response stream: %w", readErr)}
		}
	}
}

// Close shuts down the transfer for the given deal. It is the caller's responsibility to call Close after it no longer needs the transfer.
func (t *transfer) Close() {
	t.closeOnce.Do(func() {
		// cancel the context associated with the transfer
		if t.cancel != nil {
			t.cancel()
		}
		// wait for all go-routines associated with the transfer to return
		t.wg.Wait()
	})

}

func (t *transfer) Sub() chan types.TransportEvent {
	return t.eventCh
}
