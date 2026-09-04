package emoji

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/emoji"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/slack"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var saveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save one custom emoji from your current Slack workspace into the archive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		cfg, err := config.Load(viper.GetString("config"))
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if cfg.SlackRepo == "" {
			return fmt.Errorf("slack_repo is not set in config")
		}
		if cfg.Slack.Token == "" {
			return fmt.Errorf("slack.token is not set in config")
		}

		resolver, err := op.New(ctx)
		if err != nil {
			return err
		}
		token, err := resolver.Resolve(ctx, cfg.Slack.Token)
		if err != nil {
			return fmt.Errorf("resolve slack token: %w", err)
		}

		client := slack.New(token)

		list, err := client.EmojiList(ctx)
		if err != nil {
			return fmt.Errorf("list emoji: %w", err)
		}
		value, ok := list[name]
		if !ok {
			return fmt.Errorf("no emoji named %q in this workspace", name)
		}

		// auth.test is only for the manifest's source_workspace annotation;
		// don't fail the save over it.
		teamName, err := client.TeamName(ctx)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not get workspace name: %v\n", err)
		}

		manifest, err := emoji.LoadManifest(cfg.SlackRepo)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}

		entry := emoji.Entry{
			AddedAt:         time.Now().UTC(),
			SourceWorkspace: teamName,
		}

		e := slack.Emoji{Name: name, Value: value}
		if target, isAlias := e.IsAlias(); isAlias {
			entry.AliasOf = target
			fmt.Fprintf(cmd.OutOrStdout(), "%s is an alias of %s, no image to save\n", name, target)
		} else {
			filename, err := downloadEmoji(ctx, value, name, cfg.SlackRepo)
			if err != nil {
				return fmt.Errorf("download %s: %w", name, err)
			}
			entry.Filename = filename
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s -> emojis/%s\n", name, filename)
		}

		manifest[name] = entry
		if err := manifest.Save(cfg.SlackRepo); err != nil {
			return fmt.Errorf("save manifest: %w", err)
		}
		return nil
	},
}

// downloadEmoji fetches the emoji image at url and writes it into
// <repoDir>/emojis/<name>.<ext>, returning the filename (relative to emojis/).
func downloadEmoji(ctx context.Context, url, name, repoDir string) (string, error) {
	dir := emoji.EmojisDir(repoDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, url)
	}

	filename := name + extensionFor(url, resp.Header.Get("Content-Type"))

	f, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return filename, nil
}

// extensionFor prefers the extension already present in the URL (Slack's
// emoji URLs include one, e.g. .../bowtie/46ec6f2bb0.png); falls back to the
// response Content-Type if the URL has none.
func extensionFor(url, contentType string) string {
	if ext := filepath.Ext(url); ext != "" {
		return ext
	}
	if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}
