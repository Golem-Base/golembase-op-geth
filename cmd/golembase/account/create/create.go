package create

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/cmd/golembase/account/pkg/useraccount"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func Create() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "Create a new account",
		Action: func(c *cli.Context) error {
			// This creates the directories if they don't already exist
			walletPath, err := xdg.ConfigFile(useraccount.WalletPath)
			if err != nil {
				return fmt.Errorf("failed to get or create config file path: %w", err)
			}

			fmt.Println("walletPath", walletPath)

			if info, err := os.Stat(walletPath); err == nil && info.Size() != 0 {
				return fmt.Errorf("A wallet already exists at %s", walletPath)
			}

			password, err := readPassword()
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}

			ks := keystore.NewKeyStore(filepath.Dir(walletPath), keystore.StandardScryptN, keystore.StandardScryptP)
			account, err := ks.NewAccount(password)
			if err != nil {
				return fmt.Errorf("failed to create new account: %w", err)
			}

			created := account.URL.Path
			if created != walletPath {
				if err := os.Rename(created, walletPath); err != nil {
					return fmt.Errorf("failed to rename wallet file: %w", err)
				}
			}

			fmt.Println("New wallet created", walletPath)
			fmt.Println("Address:", account.Address.Hex())

			return nil
		},
	}
}

// readPassword reads a password from stdin if piped, or interactively if in a terminal
func readPassword() (string, error) {
	// Check if input is coming from a terminal
	if term.IsTerminal(int(syscall.Stdin)) {
		fmt.Print("Enter wallet password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bytePassword)), nil
	}

	// Otherwise, read from stdin (e.g., piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(password), nil
}
