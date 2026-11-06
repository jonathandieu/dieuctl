package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jonathandieu/dieuctl/internal/config"
	"github.com/jonathandieu/dieuctl/internal/op"
	"github.com/jonathandieu/dieuctl/internal/splitwise"
	"github.com/spf13/cobra"
)

func newSplitwiseClient(ctx context.Context) (*splitwise.Client, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	var resolver *op.Resolver
	token, err := resolveSplitwiseSecret(ctx, &resolver, cfg.Splitwise.APIToken)
	if err != nil {
		return nil, fmt.Errorf("splitwise.api_token: %w", err)
	}
	session, err := resolveSplitwiseSecret(ctx, &resolver, cfg.Splitwise.Session)
	if err != nil {
		return nil, fmt.Errorf("splitwise.session: %w", err)
	}
	csrf, err := resolveSplitwiseSecret(ctx, &resolver, cfg.Splitwise.CSRF)
	if err != nil {
		return nil, fmt.Errorf("splitwise.csrf: %w", err)
	}
	auth, err := splitwise.NewAuth(token, session, csrf)
	if err != nil {
		return nil, err
	}
	return splitwise.New("", auth, nil), nil
}

// resolveSplitwiseSecret pass-throughs plaintext and only shells out to 1Password
// when the value is an op:// reference.
func resolveSplitwiseSecret(ctx context.Context, resolver **op.Resolver, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, "op://") {
		return ref, nil
	}
	if *resolver == nil {
		r, err := op.New(ctx)
		if err != nil {
			return "", err
		}
		*resolver = r
	}
	return (*resolver).Resolve(ctx, ref)
}

var splitwiseCmd = &cobra.Command{
	Use:          "splitwise",
	Short:        "Automate your Splitwise account from a local session",
	SilenceUsage: true,
	Long: `Talk to the same /api/v3.0 JSON API the Splitwise dashboard uses.

Auth (first match):
  1. splitwise.api_token — personal API key, if you have one
  2. splitwise.session + splitwise.csrf — dashboard Cookie and X-CSRF-Token
     stored in 1Password item splitwise.session

See docs/splitwise.md. Do not paste cookies into chat or commit them.`,
}

var splitwiseWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Print the authenticated Splitwise user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newSplitwiseClient(cmd.Context())
		if err != nil {
			return err
		}
		u, err := c.CurrentUser(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s <%s> (id %d)\n", u.DisplayName(), u.Email, u.ID)
		return nil
	},
}

var splitwiseGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List Splitwise groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newSplitwiseClient(cmd.Context())
		if err != nil {
			return err
		}
		groups, err := c.Groups(cmd.Context())
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-10s %-24s %s\n", "ID", "NAME", "MEMBERS")
		for _, g := range groups {
			fmt.Fprintf(out, "%-10d %-24s %d\n", g.ID, truncate(g.Name, 24), len(g.Members))
		}
		return nil
	},
}

var (
	expensesGroup string
	expensesLimit int
)

var splitwiseExpensesCmd = &cobra.Command{
	Use:   "expenses",
	Short: "List recent expenses",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newSplitwiseClient(cmd.Context())
		if err != nil {
			return err
		}
		var groupID int64
		if expensesGroup != "" {
			g, err := resolveGroup(cmd, c, expensesGroup)
			if err != nil {
				return err
			}
			groupID = g.ID
		}
		exps, err := c.Expenses(cmd.Context(), groupID, expensesLimit)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%-10s %-12s %-10s %s\n", "ID", "DATE", "COST", "DESCRIPTION")
		for _, e := range exps {
			fmt.Fprintf(out, "%-10d %-12s %-10s %s\n", e.ID, trimDate(e.Date), e.Cost, e.Description)
		}
		return nil
	},
}

var (
	addCost        string
	addDescription string
	addGroup       string
	addCurrency    string
	addDetails     string
	addShares      []string
)

var splitwiseAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create an expense (equal split by default)",
	Long: `Create an expense in a group.

Equal split (authenticated user pays, group splits evenly):
  dieuctl splitwise add --cost 42.00 --description Groceries --group Apartment

Explicit shares (user_id:paid:owed, repeatable):
  dieuctl splitwise add --cost 42.00 --description Groceries --group Apartment \
    --share 111:42.00:21.00 --share 222:0:21.00`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newSplitwiseClient(cmd.Context())
		if err != nil {
			return err
		}
		g, err := resolveGroup(cmd, c, addGroup)
		if err != nil {
			return err
		}
		in := splitwise.CreateExpenseInput{
			Cost:         addCost,
			Description:  addDescription,
			Details:      addDetails,
			CurrencyCode: addCurrency,
			GroupID:      g.ID,
			SplitEqually: len(addShares) == 0,
		}
		for _, s := range addShares {
			share, err := parseShare(s)
			if err != nil {
				return err
			}
			in.Shares = append(in.Shares, share)
		}
		exp, err := c.CreateExpense(cmd.Context(), in)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "created expense %d %q %s in group %d (%s)\n",
			exp.ID, exp.Description, exp.Cost, g.ID, g.Name)
		return nil
	},
}

func resolveGroup(cmd *cobra.Command, c *splitwise.Client, q string) (splitwise.Group, error) {
	groups, err := c.Groups(cmd.Context())
	if err != nil {
		return splitwise.Group{}, err
	}
	return splitwise.FindGroup(groups, q)
}

func parseShare(s string) (splitwise.Share, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return splitwise.Share{}, fmt.Errorf("share %q must be user_id:paid:owed", s)
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return splitwise.Share{}, fmt.Errorf("share user id %q: %w", parts[0], err)
	}
	return splitwise.Share{UserID: id, PaidShare: parts[1], OwedShare: parts[2]}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func trimDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func init() {
	splitwiseExpensesCmd.Flags().StringVar(&expensesGroup, "group", "", "group id or name")
	splitwiseExpensesCmd.Flags().IntVar(&expensesLimit, "limit", 20, "max expenses to return")

	splitwiseAddCmd.Flags().StringVar(&addCost, "cost", "", "amount, two decimal places (required)")
	splitwiseAddCmd.Flags().StringVar(&addDescription, "description", "", "short description (required)")
	splitwiseAddCmd.Flags().StringVar(&addGroup, "group", "", "group id or name (required)")
	splitwiseAddCmd.Flags().StringVar(&addCurrency, "currency", "USD", "currency code")
	splitwiseAddCmd.Flags().StringVar(&addDetails, "details", "", "notes")
	splitwiseAddCmd.Flags().StringArrayVar(&addShares, "share", nil, "user_id:paid:owed (repeatable; omit for equal split)")
	_ = splitwiseAddCmd.MarkFlagRequired("cost")
	_ = splitwiseAddCmd.MarkFlagRequired("description")
	_ = splitwiseAddCmd.MarkFlagRequired("group")

	splitwiseCmd.AddCommand(splitwiseWhoamiCmd, splitwiseGroupsCmd, splitwiseExpensesCmd, splitwiseAddCmd)
	rootCmd.AddCommand(splitwiseCmd)
}
