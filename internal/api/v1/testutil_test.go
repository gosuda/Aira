package v1_test

import (
	"context"

	"github.com/google/uuid"

	"github.com/gosuda/aira/internal/domain"
	"github.com/gosuda/aira/internal/server/middleware"
)

// ---------------------------------------------------------------------------
// Context helpers — inject tenant/user/role into context for DoCtx
// ---------------------------------------------------------------------------

func tenantCtx(tenantID uuid.UUID) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenantID)
	return ctx
}

func adminCtx(tenantID uuid.UUID) context.Context {
	ctx := tenantCtx(tenantID)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserRole, "admin")
	return ctx
}

// ---------------------------------------------------------------------------
// Mock DataStore
// ---------------------------------------------------------------------------

type mockDataStore struct {
	tenants       domain.TenantRepository
	projects      domain.ProjectRepository
	tasks         domain.TaskRepository
	adrs          domain.ADRRepository
	agentSessions domain.AgentSessionRepository
}

func (m *mockDataStore) Tenants() domain.TenantRepository             { return m.tenants }
func (m *mockDataStore) Projects() domain.ProjectRepository           { return m.projects }
func (m *mockDataStore) Tasks() domain.TaskRepository                 { return m.tasks }
func (m *mockDataStore) ADRs() domain.ADRRepository                   { return m.adrs }
func (m *mockDataStore) AgentSessions() domain.AgentSessionRepository { return m.agentSessions }

// ---------------------------------------------------------------------------
// Mock TenantRepository
// ---------------------------------------------------------------------------

type mockTenantRepo struct {
	createFunc    func(ctx context.Context, t *domain.Tenant) error
	getByIDFunc   func(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	getBySlugFunc func(ctx context.Context, slug string) (*domain.Tenant, error)
	updateFunc    func(ctx context.Context, t *domain.Tenant) error
	listFunc      func(ctx context.Context) ([]*domain.Tenant, error)
}

func (m *mockTenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	return m.createFunc(ctx, t)
}

func (m *mockTenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return m.getByIDFunc(ctx, id)
}

func (m *mockTenantRepo) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	return m.getBySlugFunc(ctx, slug)
}

func (m *mockTenantRepo) Update(ctx context.Context, t *domain.Tenant) error {
	return m.updateFunc(ctx, t)
}

func (m *mockTenantRepo) List(ctx context.Context) ([]*domain.Tenant, error) {
	return m.listFunc(ctx)
}

// ---------------------------------------------------------------------------
// Mock ProjectRepository
// ---------------------------------------------------------------------------

type mockProjectRepo struct {
	createFunc  func(ctx context.Context, p *domain.Project) error
	getByIDFunc func(ctx context.Context, tenantID, id uuid.UUID) (*domain.Project, error)
	updateFunc  func(ctx context.Context, p *domain.Project) error
	listFunc    func(ctx context.Context, tenantID uuid.UUID) ([]*domain.Project, error)
	deleteFunc  func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *mockProjectRepo) Create(ctx context.Context, p *domain.Project) error {
	return m.createFunc(ctx, p)
}

func (m *mockProjectRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Project, error) {
	return m.getByIDFunc(ctx, tenantID, id)
}

func (m *mockProjectRepo) Update(ctx context.Context, p *domain.Project) error {
	return m.updateFunc(ctx, p)
}

func (m *mockProjectRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Project, error) {
	return m.listFunc(ctx, tenantID)
}

func (m *mockProjectRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return m.deleteFunc(ctx, tenantID, id)
}

// ---------------------------------------------------------------------------
// Mock TaskRepository
// ---------------------------------------------------------------------------

type mockTaskRepo struct {
	createFunc        func(ctx context.Context, t *domain.Task) error
	getByIDFunc       func(ctx context.Context, tenantID, id uuid.UUID) (*domain.Task, error)
	listByProjectFunc func(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.Task, error)
	listByStatusFunc  func(ctx context.Context, tenantID, projectID uuid.UUID, status domain.TaskStatus) ([]*domain.Task, error)
	updateStatusFunc  func(ctx context.Context, tenantID, id uuid.UUID, status domain.TaskStatus) error
	updateFunc        func(ctx context.Context, t *domain.Task) error
	deleteFunc        func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *mockTaskRepo) Create(ctx context.Context, t *domain.Task) error {
	return m.createFunc(ctx, t)
}

func (m *mockTaskRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Task, error) {
	return m.getByIDFunc(ctx, tenantID, id)
}

func (m *mockTaskRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.Task, error) {
	return m.listByProjectFunc(ctx, tenantID, projectID)
}

func (m *mockTaskRepo) ListByStatus(ctx context.Context, tenantID, projectID uuid.UUID, status domain.TaskStatus) ([]*domain.Task, error) {
	return m.listByStatusFunc(ctx, tenantID, projectID, status)
}

func (m *mockTaskRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.TaskStatus) error {
	return m.updateStatusFunc(ctx, tenantID, id, status)
}

func (m *mockTaskRepo) Update(ctx context.Context, t *domain.Task) error {
	return m.updateFunc(ctx, t)
}

func (m *mockTaskRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return m.deleteFunc(ctx, tenantID, id)
}

// ---------------------------------------------------------------------------
// Mock ADRRepository
// ---------------------------------------------------------------------------

type mockADRRepo struct {
	createFunc        func(ctx context.Context, adr *domain.ADR) error
	getByIDFunc       func(ctx context.Context, tenantID, id uuid.UUID) (*domain.ADR, error)
	listByProjectFunc func(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.ADR, error)
	updateStatusFunc  func(ctx context.Context, tenantID, id uuid.UUID, status domain.ADRStatus) error
}

func (m *mockADRRepo) Create(ctx context.Context, adr *domain.ADR) error {
	return m.createFunc(ctx, adr)
}

func (m *mockADRRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.ADR, error) {
	return m.getByIDFunc(ctx, tenantID, id)
}

func (m *mockADRRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.ADR, error) {
	return m.listByProjectFunc(ctx, tenantID, projectID)
}

func (m *mockADRRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.ADRStatus) error {
	return m.updateStatusFunc(ctx, tenantID, id, status)
}

// ---------------------------------------------------------------------------
// Mock AgentSessionRepository
// ---------------------------------------------------------------------------

type mockAgentSessionRepo struct {
	createFunc                 func(ctx context.Context, s *domain.AgentSession) error
	getByIDFunc                func(ctx context.Context, tenantID, id uuid.UUID) (*domain.AgentSession, error)
	updateStatusFunc           func(ctx context.Context, tenantID, id uuid.UUID, status domain.AgentSessionStatus) error
	updateContainerFunc        func(ctx context.Context, tenantID, id uuid.UUID, containerID, branchName string) error
	setCompletedFunc           func(ctx context.Context, id uuid.UUID, err string) error
	listByTaskFunc             func(ctx context.Context, tenantID, taskID uuid.UUID) ([]*domain.AgentSession, error)
	listByProjectFunc          func(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.AgentSession, error)
	listByProjectPaginatedFunc func(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*domain.AgentSession, error)
	countByProjectFunc         func(ctx context.Context, tenantID, projectID uuid.UUID) (int64, error)
}

func (m *mockAgentSessionRepo) Create(ctx context.Context, s *domain.AgentSession) error {
	return m.createFunc(ctx, s)
}

func (m *mockAgentSessionRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.AgentSession, error) {
	return m.getByIDFunc(ctx, tenantID, id)
}

func (m *mockAgentSessionRepo) UpdateStatus(ctx context.Context, tenantID, id uuid.UUID, status domain.AgentSessionStatus) error {
	return m.updateStatusFunc(ctx, tenantID, id, status)
}

func (m *mockAgentSessionRepo) UpdateContainer(ctx context.Context, tenantID, id uuid.UUID, containerID, branchName string) error {
	return m.updateContainerFunc(ctx, tenantID, id, containerID, branchName)
}

func (m *mockAgentSessionRepo) SetCompleted(ctx context.Context, id uuid.UUID, err string) error {
	return m.setCompletedFunc(ctx, id, err)
}

func (m *mockAgentSessionRepo) ListByTask(ctx context.Context, tenantID, taskID uuid.UUID) ([]*domain.AgentSession, error) {
	return m.listByTaskFunc(ctx, tenantID, taskID)
}

func (m *mockAgentSessionRepo) ListByProject(ctx context.Context, tenantID, projectID uuid.UUID) ([]*domain.AgentSession, error) {
	return m.listByProjectFunc(ctx, tenantID, projectID)
}

func (m *mockAgentSessionRepo) ListByProjectPaginated(ctx context.Context, tenantID, projectID uuid.UUID, limit, offset int) ([]*domain.AgentSession, error) {
	return m.listByProjectPaginatedFunc(ctx, tenantID, projectID, limit, offset)
}

func (m *mockAgentSessionRepo) CountByProject(ctx context.Context, tenantID, projectID uuid.UUID) (int64, error) {
	return m.countByProjectFunc(ctx, tenantID, projectID)
}

// ---------------------------------------------------------------------------
// Mock AuthService
// ---------------------------------------------------------------------------

type mockAuthService struct {
	registerFunc     func(ctx context.Context, tenantID uuid.UUID, email, password, name string) (*domain.User, error)
	loginFunc        func(ctx context.Context, tenantID uuid.UUID, email, password string) (string, string, error)
	refreshTokenFunc func(ctx context.Context, refreshToken string) (string, error)
}

func (m *mockAuthService) Register(ctx context.Context, tenantID uuid.UUID, email, password, name string) (*domain.User, error) {
	return m.registerFunc(ctx, tenantID, email, password, name)
}

func (m *mockAuthService) Login(ctx context.Context, tenantID uuid.UUID, email, password string) (accessToken, refreshToken string, err error) {
	return m.loginFunc(ctx, tenantID, email, password)
}

func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	return m.refreshTokenFunc(ctx, refreshToken)
}

// ---------------------------------------------------------------------------
// Mock AgentOrchestrator
// ---------------------------------------------------------------------------

type mockAgentOrchestrator struct {
	startTaskFunc     func(ctx context.Context, tenantID, taskID uuid.UUID, agentType string) (*domain.AgentSession, error)
	cancelSessionFunc func(ctx context.Context, tenantID, sessionID uuid.UUID) error
}

func (m *mockAgentOrchestrator) StartTask(ctx context.Context, tenantID, taskID uuid.UUID, agentType string) (*domain.AgentSession, error) {
	return m.startTaskFunc(ctx, tenantID, taskID, agentType)
}

func (m *mockAgentOrchestrator) CancelSession(ctx context.Context, tenantID, sessionID uuid.UUID) error {
	return m.cancelSessionFunc(ctx, tenantID, sessionID)
}
