package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// CmdOpen — `kb open (list|claim|release|merge)`
//
// Agent-first surface for the autonomous pick-up loop. See entry
// X-VUXYRR for the design and D-JNUYH6 for the tag convention.
func CmdOpen(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: kb open (list|claim|release|merge)")
	}
	verb := args[0]
	rest := args[1:]
	switch verb {
	case "list":
		return cmdOpenList(rest, stdout)
	case "claim":
		return cmdOpenClaim(rest, stdout)
	case "release":
		return cmdOpenRelease(rest, stdout)
	case "merge":
		return cmdOpenMerge(rest, stdout)
	}
	return fmt.Errorf("unknown open subcommand: %s", verb)
}

func cmdOpenList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("open-list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	role := fs.String("role", "", "filter to entries tagged skill:<role>")
	effort := fs.String("effort", "", "filter to S|M|L")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cli, err := loadClient()
	if err != nil {
		return err
	}
	q := url.Values{}
	if *role != "" {
		q.Set("role", *role)
	}
	if *effort != "" {
		q.Set("effort", *effort)
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := cli.Do(http.MethodGet, "/v1/open_work?"+q.Encode(), nil, nil, &out); err != nil {
		return err
	}
	if len(out.Items) == 0 {
		fmt.Fprintln(stdout, "(no open work)")
		return nil
	}
	for _, it := range out.Items {
		e, _ := it["Entry"].(map[string]any)
		eff, _ := it["Effort"].(string)
		fmt.Fprintf(stdout, "%s\t[effort:%s]\t%v\t%v\n",
			e["id"], eff, e["type"], e["title"])
	}
	return nil
}

func cmdOpenClaim(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("open-claim", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	entry := fs.String("entry", "", "entry ID (required)")
	role := fs.String("role", "", "your librarian role (required)")
	instance := fs.String("instance", "", "your instance ID (required)")
	effort := fs.String("effort", "", "S|M|L (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *entry == "" || *role == "" || *instance == "" {
		return errors.New("--entry, --role, --instance are required")
	}
	cli, err := loadClient()
	if err != nil {
		return err
	}
	body := map[string]any{"role": *role, "instance_id": *instance}
	if *effort != "" {
		body["effort"] = *effort
	}
	var out map[string]any
	if err := cli.Do(http.MethodPost,
		"/v1/entries/"+url.PathEscape(*entry)+"/claim",
		body, nil, &out); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Fprintln(stdout, string(b))
	return nil
}

func cmdOpenRelease(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("open-release", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	entry := fs.String("entry", "", "entry ID (required)")
	instance := fs.String("instance", "", "your instance ID (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *entry == "" || *instance == "" {
		return errors.New("--entry and --instance are required")
	}
	cli, err := loadClient()
	if err != nil {
		return err
	}
	if err := cli.Do(http.MethodPost,
		"/v1/entries/"+url.PathEscape(*entry)+"/release",
		map[string]any{"instance_id": *instance}, nil, nil); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "released:", *entry)
	return nil
}

func cmdOpenMerge(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("open-merge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	entry := fs.String("entry", "", "entry ID being merged (required)")
	instance := fs.String("instance", "", "your instance ID (required)")
	result := fs.String("result", "", "short summary of what was done")
	implEntry := fs.String("impl", "", "optional: entry ID documenting the implementation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *entry == "" || *instance == "" {
		return errors.New("--entry and --instance are required")
	}
	cli, err := loadClient()
	if err != nil {
		return err
	}
	body := map[string]any{"instance_id": *instance}
	if *result != "" {
		body["result"] = *result
	}
	if *implEntry != "" {
		body["impl_entry_id"] = *implEntry
	}
	if err := cli.Do(http.MethodPost,
		"/v1/entries/"+url.PathEscape(*entry)+"/mark_merged",
		body, nil, nil); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "merged:", *entry)
	return nil
}
