package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/ystepanoff/goppy/internal/protocol"
)

func cmdPing(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ExitOnError)
	pf := addPortFlags(fs)
	timeout := fs.Duration("timeout", 3*time.Second, "how long to wait for a pong")
	if err := fs.Parse(args); err != nil {
		return err
	}

	port, err := pf.open()
	if err != nil {
		return err
	}
	defer port.Close()

	if err := port.SetReadTimeout(*timeout); err != nil {
		return fmt.Errorf("set read timeout: %w", err)
	}

	if _, err := port.Write(protocol.Ping()); err != nil {
		return fmt.Errorf("write ping: %w", err)
	}

	pong, err := protocol.ReadPong(port)
	if err != nil {
		return fmt.Errorf("read pong: %w", err)
	}

	fmt.Printf("device address : 0x%02X\n", pong.DeviceAddress)
	fmt.Printf("sub-addr range : %d..%d (%d drives)\n",
		pong.MinSubAddress, pong.MaxSubAddress,
		int(pong.MaxSubAddress)-int(pong.MinSubAddress)+1)
	return nil
}
