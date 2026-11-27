package hostgroup

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	groups    []models.HostGroup
	hosts     []models.Host
	ruleCount int
	created   *models.HostGroup
	updated   *models.HostGroup
	deletedID string
	listErr   error
	updateErr error
}

func (f *fakeRepo) ListHostGroups(context.Context) ([]models.HostGroup, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]models.HostGroup{}, f.groups...), nil
}
func (f *fakeRepo) GetHostGroupByID(_ context.Context, id string) (*models.HostGroup, error) {
	for _, g := range f.groups {
		if g.ID == id {
			return &g, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (f *fakeRepo) CreateHostGroup(_ context.Context, g *models.HostGroup) error {
	g.ID = "new-id"
	f.created = g
	return nil
}
func (f *fakeRepo) UpdateHostGroup(_ context.Context, g *models.HostGroup) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = g
	return nil
}
func (f *fakeRepo) DeleteHostGroup(_ context.Context, id string) error { f.deletedID = id; return nil }
func (f *fakeRepo) CountAlertRulesForHostGroup(context.Context, string) (int, error) {
	return f.ruleCount, nil
}
func (f *fakeRepo) GetAllHosts(context.Context) ([]models.Host, error) { return f.hosts, nil }

func i18nKey(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.I18nKey
	}
	return ""
}

var hosts = []models.Host{
	{ID: "web1", Tags: []string{"prod", "web"}},
	{ID: "db1", Tags: []string{"prod", "db"}},
	{ID: "lab1", Tags: []string{"lab"}},
}

func TestList_ComputesMembersFromCurrentHosts(t *testing.T) {
	repo := &fakeRepo{
		groups: []models.HostGroup{
			{ID: "g1", Name: "Prod", MatchMode: models.HostGroupMatchAll, Tags: []string{"prod"}},
			{ID: "g2", Name: "Nobody", MatchMode: models.HostGroupMatchAll, Tags: []string{"none"}},
		},
		hosts: hosts,
	}
	groups, err := NewService(repo).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(groups[0].MemberIDs, []string{"web1", "db1"}) {
		t.Errorf("prod members = %v", groups[0].MemberIDs)
	}
	if groups[1].MemberIDs == nil || len(groups[1].MemberIDs) != 0 {
		t.Errorf("empty group members = %#v, want empty non-nil slice", groups[1].MemberIDs)
	}
}

func TestList_PropagatesRepoError(t *testing.T) {
	boom := errors.New("boom")
	if _, err := NewService(&fakeRepo{listErr: boom}).List(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestCreate_NormalizesAndReturnsMembers(t *testing.T) {
	repo := &fakeRepo{hosts: hosts}
	g, err := NewService(repo).Create(context.Background(), models.HostGroupRequest{
		Name: "  Web prod ", Tags: []string{" prod", "web", "PROD", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "Web prod" || g.MatchMode != models.HostGroupMatchAll {
		t.Errorf("name/mode = %q/%q", g.Name, g.MatchMode)
	}
	if !reflect.DeepEqual(g.Tags, []string{"prod", "web"}) {
		t.Errorf("tags = %v", g.Tags)
	}
	if !reflect.DeepEqual(g.MemberIDs, []string{"web1"}) {
		t.Errorf("members = %v", g.MemberIDs)
	}
	if repo.created == nil {
		t.Error("group not persisted")
	}
}

func TestCreate_ValidationCodes(t *testing.T) {
	existing := []models.HostGroup{{ID: "g1", Name: "Prod"}}
	tests := []struct {
		name string
		req  models.HostGroupRequest
		want string
	}{
		{"name required", models.HostGroupRequest{Name: "  ", Tags: []string{"a"}}, apperr.CodeHostGroupNameRequired},
		{"name too long", models.HostGroupRequest{Name: strings.Repeat("é", 101), Tags: []string{"a"}}, apperr.CodeHostGroupFieldTooLong},
		{"os filter too long", models.HostGroupRequest{Name: "x", OSContains: strings.Repeat("a", 101)}, apperr.CodeHostGroupFieldTooLong},
		{"bad match mode", models.HostGroupRequest{Name: "x", MatchMode: "none", Tags: []string{"a"}}, apperr.CodeHostGroupMatchModeInvalid},
		{"no criterion", models.HostGroupRequest{Name: "x", Tags: []string{" "}}, apperr.CodeHostGroupCriteriaRequired},
		{"name taken case-insensitively", models.HostGroupRequest{Name: "prod", Tags: []string{"a"}}, apperr.CodeHostGroupNameTaken},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{groups: existing}
			_, err := NewService(repo).Create(context.Background(), tc.req)
			if got := i18nKey(err); got != tc.want {
				t.Fatalf("code = %q (err %v), want %q", got, err, tc.want)
			}
			if repo.created != nil {
				t.Error("invalid group persisted")
			}
		})
	}
}

func TestCreate_OSFilterAloneIsEnough(t *testing.T) {
	repo := &fakeRepo{}
	g, err := NewService(repo).Create(context.Background(), models.HostGroupRequest{Name: "Debian", MatchMode: "any", OSContains: " debian "})
	if err != nil {
		t.Fatal(err)
	}
	if g.OSContains != "debian" || g.MatchMode != models.HostGroupMatchAny {
		t.Errorf("os/mode = %q/%q", g.OSContains, g.MatchMode)
	}
}

func TestUpdate(t *testing.T) {
	existing := []models.HostGroup{{ID: "g1", Name: "Prod", Tags: []string{"prod"}}, {ID: "g2", Name: "Lab", Tags: []string{"lab"}}}

	t.Run("keeps its own name", func(t *testing.T) {
		repo := &fakeRepo{groups: existing, hosts: hosts}
		g, err := NewService(repo).Update(context.Background(), "g1", models.HostGroupRequest{Name: "PROD", Tags: []string{"db"}})
		if err != nil {
			t.Fatal(err)
		}
		if repo.updated == nil || repo.updated.ID != "g1" {
			t.Fatalf("updated = %+v", repo.updated)
		}
		if !reflect.DeepEqual(g.MemberIDs, []string{"db1"}) {
			t.Errorf("members = %v", g.MemberIDs)
		}
	})
	t.Run("rejects another group's name", func(t *testing.T) {
		_, err := NewService(&fakeRepo{groups: existing}).Update(context.Background(), "g1", models.HostGroupRequest{Name: "lab", Tags: []string{"x"}})
		if i18nKey(err) != apperr.CodeHostGroupNameTaken {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unknown group", func(t *testing.T) {
		_, err := NewService(&fakeRepo{groups: existing}).Update(context.Background(), "nope", models.HostGroupRequest{Name: "x", Tags: []string{"x"}})
		if i18nKey(err) != apperr.CodeHostGroupNotFound {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("deleted between read and write", func(t *testing.T) {
		repo := &fakeRepo{groups: existing, updateErr: sql.ErrNoRows}
		_, err := NewService(repo).Update(context.Background(), "g1", models.HostGroupRequest{Name: "x", Tags: []string{"x"}})
		if i18nKey(err) != apperr.CodeHostGroupNotFound {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDelete(t *testing.T) {
	existing := []models.HostGroup{{ID: "g1", Name: "Prod"}}

	t.Run("refused while rules target it", func(t *testing.T) {
		repo := &fakeRepo{groups: existing, ruleCount: 2}
		err := NewService(repo).Delete(context.Background(), "g1")
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.I18nKey != apperr.CodeHostGroupInUse || ae.Params["count"] != "2" {
			t.Fatalf("err = %#v", err)
		}
		if repo.deletedID != "" {
			t.Error("group deleted despite referencing rules")
		}
	})
	t.Run("deletes an unused group", func(t *testing.T) {
		repo := &fakeRepo{groups: existing}
		if err := NewService(repo).Delete(context.Background(), "g1"); err != nil {
			t.Fatal(err)
		}
		if repo.deletedID != "g1" {
			t.Errorf("deleted = %q", repo.deletedID)
		}
	})
	t.Run("unknown group", func(t *testing.T) {
		if err := NewService(&fakeRepo{}).Delete(context.Background(), "nope"); i18nKey(err) != apperr.CodeHostGroupNotFound {
			t.Fatalf("err = %v", err)
		}
	})
}
