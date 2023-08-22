package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	tm "github.com/buger/goterm"

	datatransfer "github.com/filecoin-project/go-data-transfer/v2"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/urfave/cli/v2"
)

var DataTransfersCmd = &cli.Command{
	Name:  "data-transfers",
	Usage: "Manage data transfers",
	Subcommands: []*cli.Command{
		transfersListCmd,
		marketRestartTransfer,
		marketCancelTransfer,
		marketCancelRetrievalTransfer,
	},
}

var marketRestartTransfer = &cli.Command{
	Name:  "restart",
	Usage: "Force restart a stalled data transfer",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "peerid",
			Usage: "narrow to transfer with specific peer",
		},
		&cli.BoolFlag{
			Name:  "initiator",
			Usage: "specify only transfers where peer is/is not initiator",
			Value: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return cli.ShowCommandHelp(cctx, cctx.Command.Name)
		}
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		transferUint, err := strconv.ParseUint(cctx.Args().First(), 10, 64)
		if err != nil {
			return fmt.Errorf("Error reading transfer ID: %w", err)
		}
		transferID := datatransfer.TransferID(transferUint)
		initiator := cctx.Bool("initiator")
		var other peer.ID
		if pidstr := cctx.String("peerid"); pidstr != "" {
			p, err := peer.Decode(pidstr)
			if err != nil {
				return err
			}
			other = p
		} else {
			channels, err := nodeApi.MarketListDataTransfers(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, channel := range channels {
				if channel.IsInitiator == initiator && channel.TransferID == transferID {
					other = channel.OtherPeer
					found = true
					break
				}
			}
			if !found {
				return errors.New("unable to find matching data transfer")
			}
		}

		return nodeApi.MarketRestartDataTransfer(ctx, transferID, other, initiator)
	},
}

var marketCancelTransfer = &cli.Command{
	Name:  "cancel",
	Usage: "Force cancel a data transfer",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "peerid",
			Usage: "narrow to transfer with specific peer",
		},
		&cli.BoolFlag{
			Name:  "initiator",
			Usage: "specify only transfers where peer is/is not initiator",
			Value: false,
		},
		&cli.DurationFlag{
			Name:  "cancel-timeout",
			Usage: "time to wait for cancel to be sent to client",
			Value: 5 * time.Second,
		},
	},
	Action: func(cctx *cli.Context) error {
		if !cctx.Args().Present() {
			return cli.ShowCommandHelp(cctx, cctx.Command.Name)
		}
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		transferUint, err := strconv.ParseUint(cctx.Args().First(), 10, 64)
		if err != nil {
			return fmt.Errorf("Error reading transfer ID: %w", err)
		}
		transferID := datatransfer.TransferID(transferUint)
		initiator := cctx.Bool("initiator")
		var other peer.ID
		if pidstr := cctx.String("peerid"); pidstr != "" {
			p, err := peer.Decode(pidstr)
			if err != nil {
				return err
			}
			other = p
		} else {
			channels, err := nodeApi.MarketListDataTransfers(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, channel := range channels {
				if channel.IsInitiator == initiator && channel.TransferID == transferID {
					other = channel.OtherPeer
					found = true
					break
				}
			}
			if !found {
				return errors.New("unable to find matching data transfer")
			}
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, cctx.Duration("cancel-timeout"))
		defer cancel()
		return nodeApi.MarketCancelDataTransfer(timeoutCtx, transferID, other, initiator)
	},
}

var marketCancelRetrievalTransfer = &cli.Command{
	Name:  "batch-cancel",
	Usage: "Batch force cancel data transfer",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "peer-id",
			Usage: "client peer id",
		},
		&cli.StringFlag{
			Name:  "data-cid",
			Usage: "data root cid",
		},
		&cli.BoolFlag{
			Name:  "initiator",
			Usage: "specify only transfers where peer is/is not initiator",
			Value: false,
		},
		&cli.DurationFlag{
			Name:  "cancel-timeout",
			Usage: "time to wait for cancel to be sent to client",
			Value: 5 * time.Second,
		},
	},
	Action: func(cctx *cli.Context) error {
		nodeApi, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()

		ctx := ReqContext(cctx)
		params := market.RetrievalDealQueryParams{
			DiscardFailedDeal: true,
		}
		initiator := cctx.Bool("initiator")

		if cctx.IsSet("peer-id") {
			params.Receiver = cctx.String("peer-id")
		}
		if cctx.IsSet("data-cid") {
			params.PayloadCID = cctx.String("data-cid")
		}
		if len(params.Receiver) == 0 && len(params.PayloadCID) == 0 {
			return fmt.Errorf("`peer-id` and `data-cid` must be set to one")
		}

		deals, err := nodeApi.MarketListRetrievalDeals(ctx, &params)
		if err != nil {
			return err
		}
		if len(deals) == 0 {
			return fmt.Errorf("not found retrieval deal")
		}

		dealMap := make(map[datatransfer.TransferID]*market.ProviderDealState, len(deals))
		for i, deal := range deals {
			if deal.ChannelID == nil {
				continue
			}
			if deal.Status == retrievalmarket.DealStatusRejected ||
				deal.Status == retrievalmarket.DealStatusCompleted ||
				deal.Status == retrievalmarket.DealStatusErrored ||
				deal.Status == retrievalmarket.DealStatusCancelled {
				continue
			}
			dealMap[deal.ChannelID.ID] = &deals[i]
		}
		if len(dealMap) == 0 {
			return fmt.Errorf("no data transfer need cancel")
		}

		channels, err := nodeApi.MarketListDataTransfers(ctx)
		if err != nil {
			return err
		}

		pendingDeals := make([]*market.ProviderDealState, 0, len(channels))
		otherPeers := make([]peer.ID, 0, len(channels))
		for _, channel := range channels {
			if channel.Status == datatransfer.Completed || channel.Status == datatransfer.Failed ||
				channel.Status == datatransfer.Cancelled {
				continue
			}
			if deal, ok := dealMap[channel.TransferID]; channel.IsInitiator == initiator && ok {
				pendingDeals = append(pendingDeals, deal)
				otherPeers = append(otherPeers, channel.OtherPeer)
			}
		}

		fmt.Printf("find %d data transfer need to cancel\n", len(pendingDeals))

		timeout := cctx.Duration("cancel-timeout")
		for i, deal := range pendingDeals {
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			fmt.Printf("start cancel data transfer: %v\n", deal.ChannelID.ID)
			err := nodeApi.MarketCancelDataTransfer(timeoutCtx, deal.ChannelID.ID, otherPeers[i], initiator)
			if err != nil {
				fmt.Printf("cancel %d failed: %v\n", deal.ChannelID.ID, err)
			} else {
				fmt.Printf("cancel %d success\n", deal.ChannelID.ID)
			}
		}

		return nil
	},
}

var transfersListCmd = &cli.Command{
	Name:  "list",
	Usage: "List ongoing data transfers for this miner",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "print verbose transfer details",
		},
		&cli.BoolFlag{
			Name:  "color",
			Usage: "use color in display output",
			Value: true,
		},
		&cli.BoolFlag{
			Name:  "completed",
			Usage: "show completed data transfers",
		},
		&cli.BoolFlag{
			Name:  "watch",
			Usage: "watch deal updates in real-time, rather than a one time list",
		},
		&cli.BoolFlag{
			Name:  "show-failed",
			Usage: "show failed/cancelled transfers",
		},
	},
	Action: func(cctx *cli.Context) error {
		api, closer, err := NewMarketNode(cctx)
		if err != nil {
			return err
		}
		defer closer()
		ctx := ReqContext(cctx)

		channels, err := api.MarketListDataTransfers(ctx)
		if err != nil {
			return err
		}

		verbose := cctx.Bool("verbose")
		completed := cctx.Bool("completed")
		color := cctx.Bool("color")
		watch := cctx.Bool("watch")
		showFailed := cctx.Bool("show-failed")
		if watch {
			channelUpdates, err := api.MarketDataTransferUpdates(ctx)
			if err != nil {
				return err
			}

			for {
				tm.Clear() // Clear current screen

				tm.MoveCursor(1, 1)

				OutputDataTransferChannels(tm.Screen, channels, verbose, completed, color, showFailed)

				tm.Flush()

				select {
				case <-ctx.Done():
					return nil
				case channelUpdate := <-channelUpdates:
					var found bool
					for i, existing := range channels {
						if existing.TransferID == channelUpdate.TransferID &&
							existing.OtherPeer == channelUpdate.OtherPeer &&
							existing.IsSender == channelUpdate.IsSender &&
							existing.IsInitiator == channelUpdate.IsInitiator {
							channels[i] = channelUpdate
							found = true
							break
						}
					}
					if !found {
						channels = append(channels, channelUpdate)
					}
				}
			}
		}
		OutputDataTransferChannels(os.Stdout, channels, verbose, completed, color, showFailed)
		return nil
	},
}
