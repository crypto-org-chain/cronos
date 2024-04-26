package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"github.com/spf13/cobra"
)

const flagI = "i"

func DecryptMsgCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt [input]",
		Short: "Decrypt input messages to user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := cmd.Flags().GetStringArray(flagI)
			if err != nil {
				return err
			}
			var in io.Reader = os.Stdin
			var out io.Writer = os.Stdout
			if name := args[0]; name != "" && name != "-" {
				f, err := os.Open(name)
				if err != nil {
					return err
				}
				defer f.Close()
				in = f
			}
			return decrypt(inputs, in, out)
		},
	}
	cmd.Flags().StringArray(flagI, []string{}, "identity (can be repeated)")
	return cmd
}

func decrypt(inputs []string, in io.Reader, out io.Writer) error {
	identities := []age.Identity{}
	for _, input := range inputs {
		ids, err := parseIdentitiesFile(input)
		if err != nil {
			return err
		}
		identities = append(identities, ids...)
	}
	r, err := age.Decrypt(in, identities...)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		return err
	}
	return nil
}

func parseIdentitiesFile(name string) ([]age.Identity, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	b := bufio.NewReader(f)
	ids, err := parseIdentities(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %v", name, err)
	}
	return ids, nil
}

func parseIdentities(f io.Reader) ([]age.Identity, error) {
	const privateKeySizeLimit = 1 << 24 // 16 MiB
	var ids []age.Identity
	scanner := bufio.NewScanner(io.LimitReader(f, privateKeySizeLimit))
	var n int
	for scanner.Scan() {
		n++
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		i, err := age.ParseX25519Identity(line)
		if err != nil {
			return nil, fmt.Errorf("error at line %d: %v", n, err)
		}
		ids = append(ids, i)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read secret keys file: %v", err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no secret keys found")
	}
	return ids, nil
}
