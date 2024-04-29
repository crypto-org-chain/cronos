package cmd

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"github.com/spf13/cobra"
)

const (
	flagR = "r"
	flagO = "o"
)

func EncryptMsgCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encrypt [input]",
		Short: "Encrypt input messages to multiple users",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			output, err := f.GetString(flagO)
			if err != nil {
				return err
			}
			recs, err := f.GetStringArray(flagR)
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
				defer func() {
					if err := f.Close(); err != nil {
						fmt.Println("file close error", err)
					}
				}()
				in = f
			}
			if name := output; name != "" && name != "-" {
				f := newLazyOpener(name)
				defer func() {
					if err := f.Close(); err != nil {
						fmt.Println("file close error", err)
					}
				}()
				out = f
			}
			return encrypt(recs, in, out)
		},
	}
	f := cmd.Flags()
	f.StringArray(flagR, []string{}, "recipient (can be repeated)")
	f.String(flagO, "", "output to `FILE`")
	return cmd
}

func encrypt(recs []string, in io.Reader, out io.Writer) error {
	var recipients []age.Recipient
	for _, arg := range recs {
		r, err := age.ParseX25519Recipient(arg)
		if err != nil {
			return err
		}
		recipients = append(recipients, r)
	}
	w, err := age.Encrypt(out, recipients...)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

type lazyOpener struct {
	name string
	f    *os.File
	err  error
}

func newLazyOpener(name string) io.WriteCloser {
	return &lazyOpener{name: name}
}

func (l *lazyOpener) Write(p []byte) (n int, err error) {
	if l.f == nil && l.err == nil {
		l.f, l.err = os.Create(l.name)
	}
	if l.err != nil {
		return 0, l.err
	}
	return l.f.Write(p)
}

func (l *lazyOpener) Close() error {
	if l.f != nil {
		return l.f.Close()
	}
	return nil
}
