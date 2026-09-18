// Command demo is a miniature CLI that exists to show the screens this library
// renders. `just run` prints the human tree, `just run-ai` the AGENT=1 screen,
// and any arguments pass straight through: `just run user create --help`.
//
// Its command hierarchy is the one the README documents, so the examples there
// can be regenerated from a program that compiles.
package main

import (
	"fmt"
	"os"
	"strings"

	cobrahelptree "github.com/alexgorbatchev/cobra-help-tree/v2"
	"github.com/spf13/cobra"
)

func main() {
	root := newRootCommand()
	root.SetArgs(os.Args[1:])

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "mytool",
		Short: "Multi-level CLI application",
		Long:  "mytool manages user accounts and their API tokens.",
	}
	root.PersistentFlags().StringP("config", "c", "~/.config/mytool.yaml", "Path to configuration file")

	user := &cobra.Command{Use: "user", Short: "Manage user accounts"}
	user.AddCommand(&cobra.Command{Use: "create <name>", Short: "Create a new user", Args: cobra.MinimumNArgs(1), Run: report})
	user.AddCommand(&cobra.Command{Use: "delete <id>", Short: "Remove a user", Args: cobra.ExactArgs(1), Run: report})

	token := &cobra.Command{Use: "token", Short: "Manage API tokens for a user"}
	token.AddCommand(&cobra.Command{Use: "issue <user-id>", Short: "Issue a new API token", Args: cobra.ExactArgs(1), Run: report})
	token.AddCommand(&cobra.Command{Use: "revoke <token-id>", Short: "Revoke an existing API token", Args: cobra.ExactArgs(1), Run: report})
	user.AddCommand(token)

	root.AddCommand(user)
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print the version and exit", Args: cobra.NoArgs, Run: report})

	// Cobra has no field describing a positional argument, so the arguments each
	// command takes are documented here and rendered in both modes.
	catalog := cobrahelptree.TechCatalog{
		"mytool user create": {
			Args: []cobrahelptree.ArgSpec{
				{Name: "<name>", Description: "Login name for the new user"},
				{Name: "[email]", Description: "Address invitations are sent to"},
			},
			MutatesDB: true,
		},
		"mytool user delete": {
			Args:      []cobrahelptree.ArgSpec{{Name: "<id>", Description: "Numeric id of the user to remove"}},
			MutatesDB: true,
		},
		"mytool user token issue": {
			Args: []cobrahelptree.ArgSpec{{Name: "<user-id>", Description: "User the new token belongs to"}},
		},
		"mytool user token revoke": {
			Args: []cobrahelptree.ArgSpec{{Name: "<token-id>", Description: "Token to invalidate immediately"}},
		},
	}

	if err := cobrahelptree.SetupWithOptions(root, cobrahelptree.HelpOptions{Catalog: catalog}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return root
}

// report echoes the invocation rather than touching any real account, which is
// all a help-screen demo needs to do, and shows the same dual-mode split the
// help screens use.
func report(cmd *cobra.Command, args []string) {
	out := cmd.OutOrStdout()
	joined := strings.Join(args, " ")

	if cobrahelptree.IsAgentMode() {
		fmt.Fprintf(out, "command: %s\nargs: %s\n", cmd.CommandPath(), joined)
		return
	}
	fmt.Fprintf(out, "[OK] %s %s\n", cmd.CommandPath(), joined)
}
