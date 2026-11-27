// Package hostgroup is the service layer for dynamic host groups: named
// matching rules over host tags (and an optional OS filter) whose members
// are computed from the current hosts on every read, never stored.
package hostgroup

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

const (
	maxNameLength = 100
	maxOSLength   = 100
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	ListHostGroups(ctx context.Context) ([]models.HostGroup, error)
	GetHostGroupByID(ctx context.Context, id string) (*models.HostGroup, error)
	CreateHostGroup(ctx context.Context, g *models.HostGroup) error
	UpdateHostGroup(ctx context.Context, g *models.HostGroup) error
	DeleteHostGroup(ctx context.Context, id string) error
	CountAlertRulesForHostGroup(ctx context.Context, id string) (int, error)
	GetAllHosts(ctx context.Context) ([]models.Host, error)
}

// Service holds the host-group use-cases.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns every group with its current members.
func (s *Service) List(ctx context.Context) ([]models.HostGroup, error) {
	groups, err := s.repo.ListHostGroups(ctx)
	if err != nil {
		return nil, err
	}
	hosts, err := s.repo.GetAllHosts(ctx)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		groups[i].MemberIDs = memberIDs(groups[i], hosts)
	}
	return groups, nil
}

// Create validates and stores a new group.
func (s *Service) Create(ctx context.Context, req models.HostGroupRequest) (*models.HostGroup, error) {
	g, err := s.validate(ctx, "", req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateHostGroup(ctx, g); err != nil {
		return nil, err
	}
	return s.withMembers(ctx, g)
}

// Update replaces a group's definition. Rules targeting the group follow the
// new definition on the next alert evaluation.
func (s *Service) Update(ctx context.Context, id string, req models.HostGroupRequest) (*models.HostGroup, error) {
	if _, err := s.get(ctx, id); err != nil {
		return nil, err
	}
	g, err := s.validate(ctx, id, req)
	if err != nil {
		return nil, err
	}
	g.ID = id
	if err := s.repo.UpdateHostGroup(ctx, g); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound()
		}
		return nil, err
	}
	return s.withMembers(ctx, g)
}

// Delete removes a group, refusing while an alert rule still targets it: a
// rule whose group vanished would silently stop alerting.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.get(ctx, id); err != nil {
		return err
	}
	n, err := s.repo.CountAlertRulesForHostGroup(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return inUse(n)
	}
	// A rule can start targeting the group between the count and the delete:
	// the foreign key then refuses the delete, reported the same way.
	if err := s.repo.DeleteHostGroup(ctx, id); errors.Is(err, models.ErrHostGroupInUse) {
		if n, cerr := s.repo.CountAlertRulesForHostGroup(ctx, id); cerr == nil && n > 0 {
			return inUse(n)
		}
		return inUse(1)
	} else if err != nil {
		return err
	}
	return nil
}

func inUse(n int) error {
	return apperr.Conflict("host group is targeted by alert rules").
		I18n(apperr.CodeHostGroupInUse, map[string]string{"count": strconv.Itoa(n)})
}

func (s *Service) get(ctx context.Context, id string) (*models.HostGroup, error) {
	g, err := s.repo.GetHostGroupByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound()
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Service) withMembers(ctx context.Context, g *models.HostGroup) (*models.HostGroup, error) {
	hosts, err := s.repo.GetAllHosts(ctx)
	if err != nil {
		return nil, err
	}
	g.MemberIDs = memberIDs(*g, hosts)
	return g, nil
}

// validate normalizes a request into a group, checking the name (required,
// bounded, unique case-insensitively among the other groups), the match mode
// and that at least one criterion is set.
func (s *Service) validate(ctx context.Context, selfID string, req models.HostGroupRequest) (*models.HostGroup, error) {
	g := &models.HostGroup{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		MatchMode:   strings.TrimSpace(req.MatchMode),
		Tags:        models.NormalizeTags(req.Tags),
		OSContains:  strings.TrimSpace(req.OSContains),
	}
	if g.Name == "" {
		return nil, apperr.Validation("the group name is required").I18n(apperr.CodeHostGroupNameRequired, nil)
	}
	if utf8.RuneCountInString(g.Name) > maxNameLength || utf8.RuneCountInString(g.OSContains) > maxOSLength {
		return nil, apperr.Validation("the group name and OS filter must not exceed 100 characters").I18n(apperr.CodeHostGroupFieldTooLong, nil)
	}
	if g.MatchMode == "" {
		g.MatchMode = models.HostGroupMatchAll
	}
	if g.MatchMode != models.HostGroupMatchAll && g.MatchMode != models.HostGroupMatchAny {
		return nil, apperr.Validation("invalid match mode").I18n(apperr.CodeHostGroupMatchModeInvalid, nil)
	}
	if len(g.Tags) == 0 && g.OSContains == "" {
		return nil, apperr.Validation("a group needs at least one tag or an OS filter").I18n(apperr.CodeHostGroupCriteriaRequired, nil)
	}
	existing, err := s.repo.ListHostGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, other := range existing {
		if other.ID != selfID && strings.EqualFold(other.Name, g.Name) {
			return nil, apperr.Conflict("a host group with this name already exists").I18n(apperr.CodeHostGroupNameTaken, nil)
		}
	}
	return g, nil
}

func memberIDs(g models.HostGroup, hosts []models.Host) []string {
	members := g.Members(hosts)
	ids := make([]string, 0, len(members))
	for _, h := range members {
		ids = append(ids, h.ID)
	}
	return ids
}

func notFound() error {
	return apperr.NotFound("host group not found").I18n(apperr.CodeHostGroupNotFound, nil)
}
