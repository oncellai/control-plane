// Package hostclient provides a gRPC client to communicate with Host Agents.
package hostclient

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/oncellai/control-plane/internal/hostclient/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the gRPC connection to a single Host Agent.
type Client struct {
	hostID  string
	address string
	conn    *grpc.ClientConn
	client  pb.HostAgentClient
}

// Dial creates a new client connected to a Host Agent.
func Dial(hostID, address string) (*Client, error) {
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial host %s at %s: %w", hostID, address, err)
	}

	return &Client{
		hostID:  hostID,
		address: address,
		conn:    conn,
		client:  pb.NewHostAgentClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// CreateCell asks the Host Agent to create a new cell.
func (c *Client) CreateCell(ctx context.Context, req *pb.CreateCellRequest) (*pb.CellResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second) // setup() can take a while
	defer cancel()

	resp, err := c.client.CreateCell(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create cell on %s: %w", c.hostID, err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("create cell on %s: %s", c.hostID, resp.Error)
	}

	slog.Info("host agent: cell created", "host", c.hostID, "cell_id", resp.CellId, "port", resp.Port)
	return resp, nil
}

// PauseCell asks the Host Agent to pause a cell (snapshot + stop).
func (c *Client) PauseCell(ctx context.Context, cellID string) (*pb.CellResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second) // snapshot can take time
	defer cancel()

	resp, err := c.client.PauseCell(ctx, &pb.CellId{CellId: cellID})
	if err != nil {
		return nil, fmt.Errorf("pause cell %s on %s: %w", cellID, c.hostID, err)
	}

	slog.Info("host agent: cell paused", "host", c.hostID, "cell_id", cellID)
	return resp, nil
}

// ResumeCell asks the Host Agent to resume a paused cell.
func (c *Client) ResumeCell(ctx context.Context, cellID string) (*pb.CellResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second) // S3 restore may take time
	defer cancel()

	resp, err := c.client.ResumeCell(ctx, &pb.CellId{CellId: cellID})
	if err != nil {
		return nil, fmt.Errorf("resume cell %s on %s: %w", cellID, c.hostID, err)
	}

	slog.Info("host agent: cell resumed", "host", c.hostID, "cell_id", cellID, "port", resp.Port)
	return resp, nil
}

// DeleteCell asks the Host Agent to delete a cell and all its data.
func (c *Client) DeleteCell(ctx context.Context, cellID string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := c.client.DeleteCell(ctx, &pb.CellId{CellId: cellID})
	if err != nil {
		return fmt.Errorf("delete cell %s on %s: %w", cellID, c.hostID, err)
	}

	slog.Info("host agent: cell deleted", "host", c.hostID, "cell_id", cellID)
	return nil
}

// SnapshotCell asks the Host Agent to snapshot a cell to S3.
func (c *Client) SnapshotCell(ctx context.Context, cellID string) (*pb.SnapshotResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := c.client.SnapshotCell(ctx, &pb.CellId{CellId: cellID})
	if err != nil {
		return nil, fmt.Errorf("snapshot cell %s on %s: %w", cellID, c.hostID, err)
	}

	return resp, nil
}

// GetCellStatus gets the status of a cell from the Host Agent.
func (c *Client) GetCellStatus(ctx context.Context, cellID string) (*pb.CellResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetCellStatus(ctx, &pb.CellId{CellId: cellID})
}

// GetHostMetrics gets CPU, RAM, cell counts from the Host Agent.
func (c *Client) GetHostMetrics(ctx context.Context) (*pb.HostMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.GetHostMetrics(ctx, &pb.Empty{})
}

// ListCells lists all cells on this host.
func (c *Client) ListCells(ctx context.Context) (*pb.ListCellsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.client.ListCells(ctx, &pb.Empty{})
}
