package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/dirathea/sstart/internal/secrets"
)

// Runner executes subprocesses with injected secrets
type Runner struct {
	collector *secrets.Collector
	resetEnv  bool
}

// NewRunner creates a new runner instance
func NewRunner(collector *secrets.Collector, resetEnv bool) *Runner {
	return &Runner{
		collector: collector,
		resetEnv:  resetEnv,
	}
}

// Run executes a command with injected secrets
func (r *Runner) Run(ctx context.Context, providerIDs []string, command []string) error {
	// Collect secrets
	envSecrets, err := r.collector.Collect(ctx, providerIDs)
	if err != nil {
		return fmt.Errorf("failed to collect secrets: %w", err)
	}

	// Prepare environment
	env := os.Environ()
	if r.resetEnv {
		env = make([]string, 0)
	}

	// Merge secrets into environment
	for key, value := range envSecrets {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Prepare command
	if len(command) == 0 {
		return fmt.Errorf("no command specified")
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Set up process group so subprocess runs in its own process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Set up signal forwarding
	sigChan := make(chan os.Signal, 1)
	// Register for all signals that can be caught and forwarded
	// We filter out SIGCHLD as it's informational for the parent process
	signal.Notify(sigChan)

	// Goroutine to forward signals to subprocess
	go func() {
		for sig := range sigChan {
			// Don't forward SIGCHLD - it's informational for the parent about child process state changes
			if sig == syscall.SIGCHLD {
				continue
			}
			
			if cmd.Process != nil {
				// Forward the signal to the subprocess's process group
				// Negative PID sends signal to the process group
				if sysSig, ok := sig.(syscall.Signal); ok {
					// Try to send to process group first (negative PID)
					// If that fails, fall back to sending to the process directly
					if err := syscall.Kill(-cmd.Process.Pid, sysSig); err != nil {
						// If process group kill fails, try sending to process directly
						_ = cmd.Process.Signal(sig)
					}
				} else {
					// Fallback: send to process directly if not a syscall.Signal
					_ = cmd.Process.Signal(sig)
				}
			}
		}
	}()

	// Wait for command to complete
	waitErr := cmd.Wait()

	// Stop forwarding signals
	signal.Stop(sigChan)
	close(sigChan)

	if waitErr != nil {
		// Get exit code if available
		if exitError, ok := waitErr.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
				return nil
			}
		}
		return waitErr
	}

	return nil
}
