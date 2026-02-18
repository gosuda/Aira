package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/secrets"
)

type Store struct {
	pool         *pgxpool.Pool
	tenants      *TenantRepo
	users        *UserRepo
	projects     *ProjectRepo
	projectRepos *ProjectRepoRepo
	adrs         *ADRRepo
	tasks        *TaskRepo
	agents       *AgentSessionRepo
	hitl         *HITLRepo
	audit        *AuditRepo
	sessionLogs  *SessionLogRepo
	secrets      *SecretRepo
}

func New(ctx context.Context, dsn string, maxConns int32) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres.New: parse config: %w", err)
	}

	cfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres.New: connect: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres.New: ping: %w", err)
	}

	return &Store{
		pool:         pool,
		tenants:      NewTenantRepo(pool),
		users:        NewUserRepo(pool),
		projects:     NewProjectRepo(pool),
		projectRepos: NewProjectRepoRepo(pool),
		adrs:         NewADRRepo(pool),
		tasks:        NewTaskRepo(pool),
		agents:       NewAgentSessionRepo(pool),
		hitl:         NewHITLRepo(pool),
		audit:        NewAuditRepo(pool),
		sessionLogs:  NewSessionLogRepo(pool),
		secrets:      NewSecretRepo(pool),
	}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Tenants() domain.TenantRepository             { return s.tenants }
func (s *Store) Users() domain.UserRepository                 { return s.users }
func (s *Store) Projects() domain.ProjectRepository           { return s.projects }
func (s *Store) ProjectRepos() domain.ProjectRepoRepository   { return s.projectRepos }
func (s *Store) ADRs() domain.ADRRepository                   { return s.adrs }
func (s *Store) Tasks() domain.TaskRepository                 { return s.tasks }
func (s *Store) AgentSessions() domain.AgentSessionRepository { return s.agents }
func (s *Store) HITL() domain.HITLQuestionRepository          { return s.hitl }
func (s *Store) Audit() domain.AuditRepository                { return s.audit }
func (s *Store) SessionLogs() domain.SessionLogRepository     { return s.sessionLogs }
func (s *Store) Secrets() secrets.SecretRepository            { return s.secrets }
