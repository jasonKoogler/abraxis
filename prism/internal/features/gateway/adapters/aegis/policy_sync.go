package aegis

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/jasonKoogler/abraxis/aegis/api/grpc/aegispb"
	"github.com/jasonKoogler/abraxis/prism/internal/common/log"
)

// syncPoliciesLoop reconnects to the SyncPolicies stream with
// exponential backoff whenever the stream ends or errors.
func (c *Client) syncPoliciesLoop(ctx context.Context) {
	attempt := 0
	for {
		err := c.runPolicySync(ctx)
		if ctx.Err() != nil {
			return
		}

		wait := c.backoff(attempt)
		c.logger.Error("policy sync disconnected, reconnecting",
			log.Error(err),
			log.String("backoff", wait.String()),
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		attempt++
	}
}

// runPolicySync opens a single SyncPolicies stream and processes events
// until the stream ends or an error occurs. Each PolicyEvent carries a
// full policy map that is loaded into the local OPA evaluator.
func (c *Client) runPolicySync(ctx context.Context) error {
	stream, err := c.client.SyncPolicies(ctx, &pb.SyncRequest{})
	if err != nil {
		return fmt.Errorf("opening policy stream: %w", err)
	}

	c.logger.Info("policy sync stream opened")

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return fmt.Errorf("policy stream ended")
		}
		if err != nil {
			return fmt.Errorf("receiving policy event: %w", err)
		}

		if len(event.GetPolicies()) > 0 {
			if err := c.authzAdapter.UpdatePolicies(event.GetPolicies()); err != nil {
				c.logger.Error("failed to update policies", log.Error(err))
			} else {
				c.logger.Info("policies updated",
					log.String("version", event.GetVersion()),
				)
			}
		}
	}
}
