// Package clip provides clipboard access. macOS-only for now (pbcopy).
package clip

import (
	"fmt"
	"os/exec"
)

// Copy writes text to the macOS clipboard via pbcopy.
func Copy(text string) error {
	cmd := exec.Command("pbcopy")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("pbcopy stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pbcopy start: %w", err)
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		stdin.Close()
		return fmt.Errorf("pbcopy write: %w", err)
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pbcopy wait: %w", err)
	}
	return nil
}
