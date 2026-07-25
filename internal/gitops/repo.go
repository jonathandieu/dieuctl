package gitops

import (
	"errors"
	"fmt"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Repo struct {
	repo *gogit.Repository
	path string
}

// Open opens an existing git repo at path.
func Open(path string) (*Repo, error) {
	r, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open git repo at %q: %w", path, err)
	}
	return &Repo{repo: r, path: path}, nil
}

// Add stages a file relative to the repo root.
func (r *Repo) Add(relPath string) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}
	_, err = wt.Add(relPath)
	return err
}

// AddAll stages all changes (equivalent to git add -A).
func (r *Repo) AddAll() error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}
	return wt.AddWithOptions(&gogit.AddOptions{All: true})
}

// Commit creates a commit with the given message. Returns nil if there is
// nothing staged (idempotent retry-safe).
func (r *Repo) Commit(message string) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}
	status, err := wt.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		return nil
	}
	_, err = wt.Commit(message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "dieuctl",
			Email: "bot@dieu.dev",
			When:  time.Now(),
		},
	})
	if errors.Is(err, gogit.ErrEmptyCommit) {
		return nil
	}
	return err
}

// Push pushes the current branch to origin using the provided auth token.
func (r *Repo) Push(token string) error {
	err := r.repo.Push(&gogit.PushOptions{
		Auth: &gitHTTPAuth{token: token},
	})
	if err == gogit.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// gitHTTPAuth implements go-git's transport.AuthMethod for GitHub token auth.
type gitHTTPAuth struct {
	token string
}

func (a *gitHTTPAuth) Name() string   { return "http-basic-auth" }
func (a *gitHTTPAuth) String() string { return "http-basic-auth - token" }
func (a *gitHTTPAuth) SetAuth(r interface{ SetBasicAuth(string, string) }) {
	r.SetBasicAuth("x-access-token", a.token)
}
