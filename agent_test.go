package cobrahelptree

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestRenderAgentHelp(t *testing.T) {
	root := buildSampleCommandHierarchy()

	// 1. Nil check
	if got := RenderAgentHelp(nil, nil, AgentOptions{}); got != "" {
		t.Errorf("expected empty string for nil command, got %q", got)
	}

	// 2. Default agent output
	agentOut := RenderAgentHelp(root, nil, AgentOptions{})
	if !strings.Contains(agentOut, "command: app") || !strings.Contains(agentOut, "summary: Sample CLI application") {
		t.Errorf("unexpected agent help output:\n%s", agentOut)
	}
	if !strings.Contains(agentOut, "subcommands:\n  - playlist: Inspect and manage playlists") {
		t.Errorf("agent help missing subcommands:\n%s", agentOut)
	}

	// 3. With TechCatalog
	catalog := TechCatalog{
		"app": TechInfo{
			Summary:     "Overridden Summary",
			Description: "Overridden Description",
			Args:        []ArgSpec{{Name: "<required-arg>", Description: "The one required argument"}},
			MutatesDB:   true,
			AutoBackup:  true,
			Metadata: map[string]string{
				"custom_key": "custom_val",
			},
		},
	}

	catOut := RenderAgentHelp(root, catalog, AgentOptions{})
	if !strings.Contains(catOut, "summary: Overridden Summary") || !strings.Contains(catOut, "mutates_db: true") {
		t.Errorf("techCatalog not reflected in agent output:\n%s", catOut)
	}
	if !strings.Contains(catOut, "\nmetadata:\n  custom_key: custom_val\n") {
		t.Errorf("metadata not nested under a metadata: block in agent output:\n%s", catOut)
	}
}

func TestRenderAgentHelpIncludesPersistentAndInheritedFlags(t *testing.T) {
	// cobra's Flags() does not merge persistent flags; only LocalFlags() and
	// InheritedFlags() call mergePersistentFlags. Rendering straight off Flags()
	// therefore drops persistent flags whenever nothing has merged them yet,
	// which is every direct call that has not gone through Execute.
	root := &cobra.Command{Use: "app", Short: "App"}
	root.PersistentFlags().StringP("config", "c", "cfg.yaml", "Config path")

	child := &cobra.Command{Use: "child", Short: "Child"}
	child.Flags().Bool("force", false, "Force it")
	root.AddCommand(child)

	t.Run("own persistent flag on the root", func(t *testing.T) {
		out := RenderAgentHelp(root, nil, AgentOptions{})
		if !strings.Contains(out, "--config") {
			t.Errorf("persistent flag missing from agent output:\n%s", out)
		}
	})

	t.Run("inherited flag on a subcommand", func(t *testing.T) {
		out := RenderAgentHelp(child, nil, AgentOptions{})
		if !strings.Contains(out, "--force") {
			t.Errorf("local flag missing from agent output:\n%s", out)
		}
		if !strings.Contains(out, "--config") {
			t.Errorf("inherited flag missing from agent output:\n%s", out)
		}
	})

	t.Run("first render matches later ones", func(t *testing.T) {
		// Collecting flags merges cobra's persistent flag sets, which flips
		// HasAvailableFlags and so changes UseLine. Unless that happens before the
		// usage line is built, the first render differs from every later one.
		fresh := &cobra.Command{Use: "app", Short: "App"}
		fresh.PersistentFlags().String("token", "", "API token")

		first := RenderAgentHelp(fresh, nil, AgentOptions{})
		second := RenderAgentHelp(fresh, nil, AgentOptions{})
		if first != second {
			t.Errorf("render is not idempotent:\n--- first ---\n%s\n--- second ---\n%s", first, second)
		}
	})

	t.Run("agrees with the human renderer", func(t *testing.T) {
		// Both renderers must report the same set of applicable flags.
		fresh := &cobra.Command{Use: "app", Short: "App"}
		fresh.PersistentFlags().String("token", "", "API token")
		freshChild := &cobra.Command{Use: "child", Short: "Child"}
		fresh.AddCommand(freshChild)

		agent := RenderAgentHelp(freshChild, nil, AgentOptions{})
		human := RenderTreeHelp(freshChild, nil, TreeOptions{})
		if strings.Contains(human, "--token") != strings.Contains(agent, "--token") {
			t.Errorf("renderers disagree on --token\n--- agent ---\n%s\n--- human ---\n%s", agent, human)
		}
	})
}

func TestRenderersReportPflagGlobals(t *testing.T) {
	// Cobra folds pflag.CommandLine into every root's persistent flags, so a flag
	// registered on pflag's global set really is accepted by the CLI. Help must
	// therefore list it: hiding it would document an interface the binary does not
	// have. Do not "fix" this by filtering. Note this covers pflag's global set,
	// not the standard library's flag package, which cobra never consults.
	saved := pflag.CommandLine
	pflag.CommandLine = pflag.NewFlagSet("test", pflag.ContinueOnError)
	t.Cleanup(func() { pflag.CommandLine = saved })
	pflag.String("injected-global", "", "Registered on pflag.CommandLine")

	root := &cobra.Command{Use: "app", Short: "App"}
	root.Flags().String("declared", "", "Declared by the author")

	human := RenderTreeHelp(root, nil, TreeOptions{TerminalWidth: 200})
	agent := RenderAgentHelp(root, nil, AgentOptions{})

	for name, out := range map[string]string{"human": human, "agent": agent} {
		if !strings.Contains(out, "--declared") {
			t.Errorf("%s output dropped the author's own flag:\n%s", name, out)
		}
		if !strings.Contains(out, "--injected-global") {
			t.Errorf("%s output hid a pflag global the CLI actually accepts:\n%s", name, out)
		}
	}
}

func TestRenderAgentHelpIsUnclippedByDefault(t *testing.T) {
	// The agent-mode contract is full untruncated content, so neither a narrow
	// terminal nor $COLUMNS may clip it. Only an explicit MaxLineWidth can.
	t.Setenv("COLUMNS", "20")
	withNonTerminalStdout(t)

	long := strings.Repeat("x", 300)
	root := &cobra.Command{Use: "app", Short: long}

	out := RenderAgentHelp(root, nil, AgentOptions{})
	if !strings.Contains(out, long) {
		t.Errorf("agent output was clipped without an explicit MaxLineWidth:\n%s", out)
	}
	if strings.Contains(out, ellipsis) {
		t.Errorf("agent output contains an ellipsis by default:\n%s", out)
	}
}

func TestRenderAgentHelpMaxLineWidth(t *testing.T) {
	const maxWidth = 40

	root := &cobra.Command{Use: "app", Short: strings.Repeat("x", 300)}
	catalog := TechCatalog{
		"app": TechInfo{
			// A wide-character value confirms clipping counts cells, not runes.
			Metadata: map[string]string{"note": strings.Repeat("日", 60)},
		},
	}

	out := RenderAgentHelp(root, catalog, AgentOptions{MaxLineWidth: maxWidth})

	// Every key in this fixture is narrow enough to leave room for a value, so
	// every line fits. The case where a key is too wide to clip around is covered
	// by TestRenderAgentHelpClipNeverStrandsAKey.
	var clipped int
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if w := runewidth.StringWidth(line); w > maxWidth {
			t.Errorf("line occupies %d cells, exceeding MaxLineWidth %d: %q", w, maxWidth, line)
		}
		if strings.HasSuffix(line, ellipsis) {
			clipped++
		}
	}
	if clipped < 2 {
		t.Errorf("expected the long summary and the wide metadata value to be clipped, got %d clipped lines:\n%s", clipped, out)
	}
}

func TestRenderAgentHelpClipNeverStrandsAKey(t *testing.T) {
	// Agent output is parsed by machine, so clipping must not leave a key whose
	// value is nothing but an ellipsis: that reads as a field while carrying no
	// recoverable data. Keys too wide to leave room for a value stay unclipped.
	const maxWidth = 24
	longValue := strings.Repeat("v", 80)

	root := &cobra.Command{Use: "app", Short: "App"}
	catalog := TechCatalog{
		"app": TechInfo{
			Metadata: map[string]string{
				"note":                          longValue, // room for a value
				"dangling_key_here":             longValue, // key + ellipsis exactly fills the budget
				"a_very_long_metadata_key_name": longValue, // key alone overruns the budget
			},
		},
	}

	out := RenderAgentHelp(root, catalog, AgentOptions{MaxLineWidth: maxWidth})

	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.HasSuffix(line, ellipsis) {
			continue
		}
		sep := strings.Index(line, ": ")
		if sep < 0 {
			t.Errorf("clipped a line carrying no key: %q", line)
			continue
		}
		value := strings.TrimSuffix(line[sep+2:], ellipsis)
		if strings.TrimSpace(value) == "" {
			t.Errorf("clipping stranded key %q with no value: %q", line[:sep], line)
		}
	}

	// The two wide keys keep their full value rather than becoming dangling keys.
	for _, key := range []string{"dangling_key_here", "a_very_long_metadata_key_name"} {
		want := "  " + key + ": " + longValue
		if !strings.Contains(out, want) {
			t.Errorf("expected %q to stay unclipped, got:\n%s", key, out)
		}
	}

	// A key with room to spare is still clipped, so the option keeps working.
	noteClipped := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  note: ") {
			noteClipped = strings.HasSuffix(line, ellipsis) && runewidth.StringWidth(line) <= maxWidth
		}
	}
	if !noteClipped {
		t.Errorf("expected the narrow key to be clipped within %d cells:\n%s", maxWidth, out)
	}
}

func TestRenderAgentHelpClipPreservesBlockHeaders(t *testing.T) {
	// A block opener carries no value to shorten, so clipping one would destroy
	// the structure of the document rather than trim a field. Even an absurdly
	// narrow MaxLineWidth must leave these intact.
	root := &cobra.Command{Use: "app", Short: "App"}
	root.Flags().Bool("x", false, "X")
	root.AddCommand(leafCommand("sub", "Sub"))
	catalog := TechCatalog{"app": TechInfo{Metadata: map[string]string{"k": "v"}}}

	out := RenderAgentHelp(root, catalog, AgentOptions{MaxLineWidth: 6})

	for _, header := range []string{"metadata:", "subcommands:", "flags:"} {
		if !strings.Contains(out, "\n"+header+"\n") {
			t.Errorf("block header %q was clipped away:\n%s", header, out)
		}
	}
}

func TestRenderAgentHelpMetadataIsNested(t *testing.T) {
	root := &cobra.Command{Use: "app", Short: "Sample CLI application"}
	catalog := TechCatalog{
		"app": TechInfo{
			Summary: "Sample CLI application",
			// "usage" and "summary" deliberately collide with reserved top-level keys.
			Metadata: map[string]string{
				"usage":   "42 calls/min",
				"summary": "quota exhausted",
				"region":  "us-east-1",
			},
		},
	}

	out := RenderAgentHelp(root, catalog, AgentOptions{})

	if !strings.Contains(out, "\nmetadata:\n") {
		t.Fatalf("expected a nested metadata: block, got:\n%s", out)
	}

	nested := []string{
		"\n  region: us-east-1\n",
		"\n  summary: quota exhausted\n",
		"\n  usage: 42 calls/min\n",
	}
	for _, want := range nested {
		if !strings.Contains(out, want) {
			t.Errorf("expected nested metadata entry %q, got:\n%s", want, out)
		}
	}

	// Nesting exists so a metadata key can never shadow or duplicate a reserved key.
	for _, key := range []string{"command", "summary", "usage"} {
		if n := countTopLevelKeys(out, key); n != 1 {
			t.Errorf("expected exactly one top-level %q line, got %d in:\n%s", key, n, out)
		}
	}
	if strings.Contains(out, "\nusage: 42 calls/min\n") {
		t.Errorf("metadata value leaked into the top-level usage key:\n%s", out)
	}
	if strings.Contains(out, "\nsummary: quota exhausted\n") {
		t.Errorf("metadata value leaked into the top-level summary key:\n%s", out)
	}
}

func TestRenderAgentHelpMetadataIsDeterministic(t *testing.T) {
	// Keys are declared out of alphabetical order so a sorted render is not
	// accidentally satisfied by insertion order.
	sortedKeys := []string{"alpha_key", "beta_key", "delta_key", "gamma_key", "zeta_key"}
	catalog := TechCatalog{
		"app": TechInfo{
			Summary: "Sample CLI application",
			Metadata: map[string]string{
				"zeta_key":  "5",
				"gamma_key": "3",
				"alpha_key": "1",
				"delta_key": "4",
				"beta_key":  "2",
			},
		},
	}

	root := buildSampleCommandHierarchy()
	first := RenderAgentHelp(root, catalog, AgentOptions{})

	// Go randomizes map iteration per range statement, so repeated renders of the
	// same input surface any ordering instability with near certainty.
	const renders = 50
	for i := 1; i < renders; i++ {
		if got := RenderAgentHelp(root, catalog, AgentOptions{}); got != first {
			t.Fatalf("render %d differs from render 0:\n--- first ---\n%s\n--- got ---\n%s", i, first, got)
		}
	}

	prev := -1
	for _, k := range sortedKeys {
		at := strings.Index(first, "\n  "+k+": ")
		if at < 0 {
			t.Fatalf("metadata key %q missing from output:\n%s", k, first)
		}
		if at < prev {
			t.Errorf("metadata key %q is out of alphabetical order:\n%s", k, first)
		}
		prev = at
	}
}

func TestRenderAgentHelpRendersArguments(t *testing.T) {
	root := buildSampleCommandHierarchy()
	create := findCommand(t, root, "app playlist create")
	cat := TechCatalog{
		"app playlist create": TechInfo{Args: []ArgSpec{
			{Name: "<name>", Description: "Name of the new playlist"},
			{Name: "[parent]"},
		}},
	}

	out := RenderAgentHelp(create, cat, AgentOptions{})

	want := "args:\n  - <name>: Name of the new playlist\n  - [parent]\n"
	if !strings.Contains(out, want) {
		t.Errorf("agent output missing the args block %q:\n%s", want, out)
	}
}

func TestRenderAgentHelpReportsValidArgsSeparately(t *testing.T) {
	// ValidArgs is cobra's enum of accepted values for the first positional
	// argument, not the argument list, and CompletionWithDesc packs a description
	// behind a tab.
	cmd := &cobra.Command{
		Use:       "create <name>",
		Short:     "Create a playlist",
		ValidArgs: []cobra.Completion{cobra.CompletionWithDesc("alpha", "the alpha kind"), "beta"},
	}
	cat := TechCatalog{"create": TechInfo{Args: []ArgSpec{{Name: "<name>", Description: "Name of the new playlist"}}}}

	out := RenderAgentHelp(cmd, cat, AgentOptions{})

	if strings.Contains(out, "\t") {
		t.Errorf("agent output leaks a raw tab from ValidArgs:\n%q", out)
	}
	want := "valid_args:\n  - alpha: the alpha kind\n  - beta\n"
	if !strings.Contains(out, want) {
		t.Errorf("agent output missing the valid_args block %q:\n%s", want, out)
	}
	if !strings.Contains(out, "args:\n  - <name>: Name of the new playlist\n") {
		t.Errorf("catalog arguments must stay under args:, not valid_args::\n%s", out)
	}
}

func TestRenderAgentHelpReportsDeprecation(t *testing.T) {
	old := &cobra.Command{
		Use:        "old <id>",
		Short:      "A superseded command",
		Deprecated: "use \"app new\" instead",
		Run:        func(*cobra.Command, []string) {},
	}

	out := RenderAgentHelp(old, nil, AgentOptions{})
	if want := "deprecated: use \"app new\" instead\n"; !strings.Contains(out, want) {
		t.Errorf("agent output missing %q:\n%s", want, out)
	}

	current := &cobra.Command{Use: "new", Short: "The replacement", Run: func(*cobra.Command, []string) {}}
	if got := RenderAgentHelp(current, nil, AgentOptions{}); strings.Contains(got, "deprecated") {
		t.Errorf("a command that is not deprecated must carry no deprecated key:\n%s", got)
	}
}
