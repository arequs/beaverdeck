package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigSnapshotExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	source, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	if err := source.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := source.PrepareBootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Initialized {
		t.Fatalf("expected pending bootstrap, got initialized=%v", status.Initialized)
	}
	if err := source.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := source.CreateRole(ctx, "ops", "viewer", []byte(`{"namespaces":["prod"],"resources":{"pods":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := source.Create(ctx, "operator", "operator-pass", Role("ops")); err != nil {
		t.Fatal(err)
	}
	if err := source.UpdateGoogleConfig(ctx, GoogleConfig{
		ClientID:            "google-client",
		ClientSecret:        "google-secret",
		HostedDomain:        "Example.COM",
		ServiceAccountJSON:  `{"type":"service_account"}`,
		DelegatedAdminEmail: "Admin@Example.COM",
	}); err != nil {
		t.Fatal(err)
	}
	if err := source.UpsertGoogleGroupRole(ctx, "Team@Example.COM", Role("ops")); err != nil {
		t.Fatal(err)
	}
	if err := source.UpdateOIDCConfig(ctx, OIDCConfig{
		ProviderName: "Corporate OIDC",
		IssuerURL:    "https://id.example.com",
		ClientID:     "oidc-client",
		ClientSecret: "oidc-secret",
		Scopes:       "openid email profile groups",
		GroupsClaim:  "groups",
	}); err != nil {
		t.Fatal(err)
	}
	if err := source.UpsertOIDCGroupRole(ctx, "oidc-operators", Role("ops")); err != nil {
		t.Fatal(err)
	}
	if err := source.UpdateEntraConfig(ctx, OIDCConfig{
		ProviderName: "Azure Entra ID",
		IssuerURL:    "https://login.microsoftonline.com/example/v2.0",
		ClientID:     "entra-client",
		ClientSecret: "entra-secret",
		Scopes:       "openid email profile User.Read GroupMember.Read.All",
		GroupsClaim:  "groups",
	}); err != nil {
		t.Fatal(err)
	}
	if err := source.UpsertEntraGroupRole(ctx, "entra-group-id", RoleAdmin); err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.ExportConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Initialized {
		t.Fatal("expected initialized export")
	}
	if len(snapshot.Users) != 2 {
		t.Fatalf("expected 2 local users, got %d", len(snapshot.Users))
	}
	assertAdminRoleHasNoExplicitPermissions(t, snapshot.Roles)
	assertRoleAbsent(t, snapshot.Roles, "viewer")
	for _, user := range snapshot.Users {
		if !strings.HasPrefix(user.PasswordHash, localPasswordHashPrefix+"$") {
			t.Fatalf("user %s exported without bdk1 password hash", user.Username)
		}
		if strings.Contains(user.PasswordHash, "admin-pass") || strings.Contains(user.PasswordHash, "operator-pass") {
			t.Fatalf("user %s export leaked raw password", user.Username)
		}
	}

	data, err := EncodeConfigSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "schema_version:") || strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		t.Fatalf("expected YAML config export, got: %s", string(data))
	}
	if strings.Contains(string(data), "bootstrap_token") || strings.Contains(string(data), "suppressed_insights") {
		t.Fatalf("encoded config should not contain bootstrap/suppression state: %s", string(data))
	}
	if strings.Contains(string(data), "created_at:") {
		t.Fatalf("encoded config should not contain creation timestamps: %s", string(data))
	}
	if !strings.Contains(string(data), "pods: view") {
		t.Fatalf("expected compact permission level in YAML config, got: %s", string(data))
	}
	if strings.Contains(string(data), "view: true") || strings.Contains(string(data), "edit: false") || strings.Contains(string(data), "delete: false") {
		t.Fatalf("encoded config should contain compact permission levels, not permission booleans: %s", string(data))
	}
	decoded, err := DecodeConfigSnapshotBytes(data)
	if err != nil {
		t.Fatal(err)
	}

	target, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := target.ImportConfigSnapshot(ctx, decoded); err != nil {
		t.Fatal(err)
	}
	if _, err := target.VerifyLocalCredentials(ctx, "admin", "admin-pass"); err != nil {
		t.Fatalf("imported admin credentials do not verify: %v", err)
	}
	if _, err := target.VerifyLocalCredentials(ctx, "operator", "operator-pass"); err != nil {
		t.Fatalf("imported operator credentials do not verify: %v", err)
	}
	googleMappings, err := target.ListGoogleGroupRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(googleMappings) != 1 || googleMappings[0].GroupEmail != "team@example.com" {
		t.Fatalf("unexpected google mappings: %#v", googleMappings)
	}
	oidcMappings, err := target.ListOIDCGroupRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(oidcMappings) != 1 || oidcMappings[0].GroupName != "oidc-operators" {
		t.Fatalf("unexpected oidc mappings: %#v", oidcMappings)
	}
	entraMappings, err := target.ListEntraGroupRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entraMappings) != 1 || entraMappings[0].GroupName != "entra-group-id" {
		t.Fatalf("unexpected entra mappings: %#v", entraMappings)
	}
	entraToken, err := target.CreateExternalSession(ctx, "Entra@Example.COM", "entra", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	entraUser, err := target.Authenticate(ctx, entraToken)
	if err != nil {
		t.Fatal(err)
	}
	if entraUser.Username != "entra@example.com" || entraUser.AuthSource != "entra" {
		t.Fatalf("unexpected Entra session user: %#v", entraUser)
	}
	externalToken, err := target.CreateExternalSession(ctx, "External@Example.COM", "oidc", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	externalUser, err := target.Authenticate(ctx, externalToken)
	if err != nil {
		t.Fatal(err)
	}
	if externalUser.Username != "external@example.com" || externalUser.AuthSource != "oidc" {
		t.Fatalf("unexpected external session user: %#v", externalUser)
	}
	listedUsers, err := target.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listedUsers) != 2 {
		t.Fatalf("external session user should not be listed as configured user: %#v", listedUsers)
	}
	afterExternalLogin, err := target.ExportConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterExternalLogin.Users) != 2 {
		t.Fatalf("external session user should not be exported: %#v", afterExternalLogin.Users)
	}
}

func TestOIDCAndEntraConfigurationsAreIndependent(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}

	oidcConfig := OIDCConfig{
		ProviderName: "Corporate OIDC",
		IssuerURL:    "https://id.example.com",
		ClientID:     "oidc-client",
		ClientSecret: "oidc-secret",
	}
	entraConfig := OIDCConfig{
		ProviderName: "Azure Entra ID",
		IssuerURL:    "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:     "entra-client",
		ClientSecret: "entra-secret",
	}
	if err := store.UpdateOIDCConfig(ctx, oidcConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateEntraConfig(ctx, entraConfig); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertOIDCGroupRole(ctx, "oidc-admins", RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertEntraGroupRole(ctx, "entra-admins", RoleAdmin); err != nil {
		t.Fatal(err)
	}

	gotOIDC, err := store.GetOIDCConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotEntra, err := store.GetEntraConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if gotOIDC.ClientID != "oidc-client" || gotOIDC.IssuerURL != oidcConfig.IssuerURL {
		t.Fatalf("unexpected OIDC config: %#v", gotOIDC)
	}
	if gotEntra.ClientID != "entra-client" || gotEntra.IssuerURL != entraConfig.IssuerURL {
		t.Fatalf("unexpected Entra config: %#v", gotEntra)
	}
	if _, _, err := store.ResolveOIDCRole(ctx, []string{"entra-admins"}); err == nil {
		t.Fatal("generic OIDC must not use Entra mappings")
	}
	if _, _, err := store.ResolveEntraRole(ctx, []string{"oidc-admins"}); err == nil {
		t.Fatal("Entra must not use generic OIDC mappings")
	}

	if err := store.ResetOIDCAuth(ctx); err != nil {
		t.Fatal(err)
	}
	gotEntra, err = store.GetEntraConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if gotEntra.ClientID != "entra-client" {
		t.Fatalf("resetting OIDC changed Entra config: %#v", gotEntra)
	}
	entraMappings, err := store.ListEntraGroupRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entraMappings) != 1 || entraMappings[0].GroupName != "entra-admins" {
		t.Fatalf("resetting OIDC changed Entra mappings: %#v", entraMappings)
	}

	snapshot, err := store.ExportConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.OIDC.Config.ClientID != "" || snapshot.Entra.Config.ClientID != "entra-client" {
		t.Fatalf("export did not preserve independent provider sections: %#v", snapshot)
	}
}

func TestConfigMutationRollsBackWhenPersistenceFails(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "old-password"); err != nil {
		t.Fatal(err)
	}
	token, err := store.CreateSession(ctx, "admin", "local")
	if err != nil {
		t.Fatal(err)
	}
	store.SetConfigSaver(func(context.Context, ConfigSnapshot) error {
		return errors.New("secret update failed")
	})

	if err := store.CreateRole(ctx, "ops", "viewer", []byte(`{}`)); err == nil {
		t.Fatal("expected role persistence failure")
	}
	roles, err := store.ListRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertRoleAbsent(t, roles, "ops")

	if err := store.ResetLocalPassword(ctx, "admin", "new-password"); err == nil {
		t.Fatal("expected password persistence failure")
	}
	if _, err := store.Authenticate(ctx, token); err != nil {
		t.Fatalf("failed password update must not revoke the previous session: %v", err)
	}
	if _, err := store.VerifyLocalCredentials(ctx, "admin", "old-password"); err != nil {
		t.Fatalf("failed password update must restore the previous password: %v", err)
	}
	if _, err := store.VerifyLocalCredentials(ctx, "admin", "new-password"); err == nil {
		t.Fatal("failed password update exposed the new password")
	}
}

func TestConfigMutationsArePersistedSequentially(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-password"); err != nil {
		t.Fatal(err)
	}

	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var once sync.Once
	store.SetConfigSaver(func(_ context.Context, snapshot ConfigSnapshot) error {
		switch snapshot.Google.Config.ClientID {
		case "first":
			once.Do(func() { close(firstStarted) })
			<-releaseFirst
		case "second":
			close(secondStarted)
		}
		return nil
	})

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- store.UpdateGoogleConfig(ctx, GoogleConfig{ClientID: "first"})
	}()
	<-firstStarted
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- store.UpdateGoogleConfig(ctx, GoogleConfig{ClientID: "second"})
	}()
	select {
	case <-secondStarted:
		t.Fatal("second persistence started before the first one completed")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	config, err := store.GetGoogleConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if config.ClientID != "second" {
		t.Fatalf("google client id = %q, want second", config.ClientID)
	}
}

func TestRoleUpdateAppliesToExistingAndNewSessions(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.ResetToEmptyConfig(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "ops", "viewer", []byte(`{"resources":{"workloads":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", Role("ops")); err != nil {
		t.Fatal(err)
	}

	token, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}
	assertSessionWorkloadEdit := func(token string, want bool) {
		t.Helper()
		user, authErr := store.Authenticate(ctx, token)
		if authErr != nil {
			t.Fatal(authErr)
		}
		permissions, parseErr := parseRolePermissionsJSON(user.Permissions)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if got := permissions.Resources["workloads"].Edit; got != want {
			t.Fatalf("workloads edit permission = %v, want %v", got, want)
		}
	}

	assertSessionWorkloadEdit(token, false)
	if err := store.UpdateRole(ctx, "ops", "viewer", []byte(`{"resources":{"workloads":{"edit":true}}}`)); err != nil {
		t.Fatal(err)
	}
	assertSessionWorkloadEdit(token, true)

	if _, err := store.VerifyLocalCredentials(ctx, "operator", "operator-pass"); err != nil {
		t.Fatal(err)
	}
	newToken, err := store.CreateSession(ctx, "operator", "local")
	if err != nil {
		t.Fatal(err)
	}
	assertSessionWorkloadEdit(newToken, true)
}

func TestConfigSnapshotCompactPermissionsAndMissingMeansNoAccess(t *testing.T) {
	adminHash, err := hashLocalPassword("admin-pass")
	if err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`
initialized: true
roles:
  - name: admin
    mode: admin
  - name: dev
    mode: viewer
    permissions:
      namespaces:
        - apps
      resources:
        clusterroles: view
        workloads: edit
        secrets: full
  - name: noaccess
    mode: viewer
    permissions:
      namespaces:
        - apps
users:
  - username: admin
    role: admin
    password_hash: %q
google:
  config: {}
  mappings: []
oidc:
  config: {}
  mappings: []
`, adminHash)

	snapshot, err := DecodeConfigSnapshotBytes([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizeConfigSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}

	dev := findRole(t, normalized.Roles, "dev")
	devPerms, err := parseRolePermissionsJSON(dev.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	if len(devPerms.Namespaces) != 1 || devPerms.Namespaces[0] != "apps" {
		t.Fatalf("unexpected namespaces: %#v", devPerms.Namespaces)
	}
	if got := devPerms.Resources["clusterroles"]; !got.View || got.Edit || got.Delete {
		t.Fatalf("clusterroles should be view-only: %#v", got)
	}
	if got := devPerms.Resources["workloads"]; !got.View || !got.Edit || got.Delete {
		t.Fatalf("workloads should be edit level: %#v", got)
	}
	if got := devPerms.Resources["secrets"]; !got.View || !got.Edit || !got.Delete {
		t.Fatalf("secrets should be full level: %#v", got)
	}

	noAccess := findRole(t, normalized.Roles, "noaccess")
	noAccessPerms, err := parseRolePermissionsJSON(noAccess.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	if len(noAccessPerms.Namespaces) != 0 || len(noAccessPerms.Resources) != 0 {
		t.Fatalf("missing permissions must mean no access, got %#v", noAccessPerms)
	}

	data, err := EncodeConfigSnapshot(normalized)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "noaccess:\n") || strings.Contains(string(data), "permissions: {}") {
		t.Fatalf("empty permissions should be omitted from YAML config: %s", string(data))
	}
	if !strings.Contains(string(data), "clusterroles: view") || !strings.Contains(string(data), "workloads: edit") || !strings.Contains(string(data), "secrets: full") {
		t.Fatalf("expected compact permission levels in YAML config: %s", string(data))
	}
}

func TestConfigSnapshotRejectsRawPassword(t *testing.T) {
	_, err := NormalizeConfigSnapshot(ConfigSnapshot{
		SchemaVersion: ConfigSnapshotSchemaVersion,
		Initialized:   true,
		Roles: []RoleDef{
			{Name: "admin", Mode: "admin", Permissions: []byte(`{}`)},
		},
		Users: []ConfigUser{
			{Username: "admin", Role: RoleAdmin, PasswordHash: "plain-password"},
		},
	})
	if err == nil {
		t.Fatal("expected raw password import to fail")
	}
	if stage := ConfigImportStage(err); stage != "users" {
		t.Fatalf("expected users stage, got %q: %v", stage, err)
	}
}

func TestStoreDoesNotKeepAuthConfigInSQLite(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "ops", "viewer", []byte(`{"resources":{"pods":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, "operator", "operator-pass", Role("ops")); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateGoogleConfig(ctx, GoogleConfig{ClientID: "google-client", ClientSecret: "google-secret"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertOIDCGroupRole(ctx, "entra-group-id", RoleAdmin); err != nil {
		t.Fatal(err)
	}

	rows, err := store.db.QueryContext(ctx, `
SELECT name
  FROM sqlite_master
 WHERE type = 'table'
   AND name IN ('users', 'roles', 'google_config', 'google_group_roles', 'oidc_config', 'oidc_group_roles', 'sessions', 'external_sessions')
 ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(tables) > 0 {
		t.Fatalf("auth config tables must not exist in SQLite: %s", strings.Join(tables, ", "))
	}
}

func TestPartialConfigSnapshotIsCompletedAfterStartupBootstrap(t *testing.T) {
	ctx := context.Background()
	adminHash, err := hashLocalPassword("admin-pass")
	if err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`
users:
  - username: admin
    role: admin
    password_hash: %q
oidc:
  config:
    issuer_url: https://login.microsoftonline.com/example/v2.0
    client_id: oidc-client
    client_secret: oidc-secret
  mappings:
    - group_name: legacy-entra-admins
      role: admin
`, adminHash)
	snapshot, err := DecodeConfigSnapshotBytes([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}

	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.ImportConfigSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	status, err := store.PrepareBootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Initialized {
		t.Fatalf("expected imported admin account to initialize bootstrap, got initialized=%v", status.Initialized)
	}

	completed, err := store.ExportConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if completed.SchemaVersion != ConfigSnapshotSchemaVersion {
		t.Fatalf("expected schema_version to be completed, got %d", completed.SchemaVersion)
	}
	if !completed.Initialized {
		t.Fatal("expected completed config to be initialized")
	}
	assertAdminRoleHasNoExplicitPermissions(t, completed.Roles)
	assertRoleAbsent(t, completed.Roles, "viewer")
	if len(completed.Users) != 1 || completed.Users[0].Username != "admin" {
		t.Fatalf("unexpected completed users: %#v", completed.Users)
	}
	if completed.OIDC.Config.ProviderName != "OpenID Connect" {
		t.Fatalf("expected default oidc provider name, got %q", completed.OIDC.Config.ProviderName)
	}
	if completed.OIDC.Config.Scopes != defaultOIDCScopes {
		t.Fatalf("expected default oidc scopes, got %q", completed.OIDC.Config.Scopes)
	}
	if completed.OIDC.Config.EmailClaim != defaultOIDCEmailClaim || completed.OIDC.Config.GroupsClaim != defaultOIDCGroupsClaim {
		t.Fatalf("expected default oidc claims, got email=%q groups=%q", completed.OIDC.Config.EmailClaim, completed.OIDC.Config.GroupsClaim)
	}
	if completed.OIDC.Config.ClientID != "" || completed.OIDC.Config.ClientSecret != "" {
		t.Fatalf("legacy Entra config must not remain in generic OIDC: %#v", completed.OIDC.Config)
	}
	if completed.Entra.Config.ClientID != "oidc-client" || completed.Entra.Config.ClientSecret != "oidc-secret" {
		t.Fatalf("legacy Entra config was not migrated: %#v", completed.Entra.Config)
	}
	if completed.Entra.Config.ProviderName != "Azure Entra ID" || completed.Entra.Config.Scopes != defaultEntraScopes {
		t.Fatalf("legacy Entra defaults were not completed: %#v", completed.Entra.Config)
	}
	if len(completed.OIDC.Mappings) != 0 || len(completed.Entra.Mappings) != 1 || completed.Entra.Mappings[0].GroupName != "legacy-entra-admins" {
		t.Fatalf("legacy Entra mappings were not migrated independently: oidc=%#v entra=%#v", completed.OIDC.Mappings, completed.Entra.Mappings)
	}
	googleConfig := completed.Google.Config
	googleConfig.UpdatedAt = ""
	if googleConfig != (GoogleConfig{}) {
		t.Fatalf("expected google config to be completed empty, got %#v", completed.Google.Config)
	}
}

func TestExplicitEntraSectionKeepsMicrosoftIssuerInGenericOIDC(t *testing.T) {
	raw := `
oidc:
  config:
    provider_name: Microsoft-compatible OIDC
    issuer_url: https://login.microsoftonline.com/example/v2.0
    client_id: generic-client
    client_secret: generic-secret
  mappings: []
entra:
  config: {}
  mappings: []
`
	snapshot, err := DecodeConfigSnapshotBytes([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizeConfigSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.OIDC.Config.ClientID != "generic-client" || normalized.OIDC.Config.ClientSecret != "generic-secret" {
		t.Fatalf("explicit generic OIDC config was moved unexpectedly: %#v", normalized.OIDC.Config)
	}
	if normalized.Entra.Config.ClientID != "" || normalized.Entra.Config.ClientSecret != "" {
		t.Fatalf("explicit empty Entra section must stay independent: %#v", normalized.Entra.Config)
	}
}

func TestImportConfigSnapshotOverwritesGoogleAndOIDCConfig(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.CompleteBootstrap(ctx, "admin", "admin-pass"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRole(ctx, "ops", "viewer", []byte(`{"resources":{"pods":{"view":true}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateGoogleConfig(ctx, GoogleConfig{
		ClientID:            "google-client-old",
		ClientSecret:        "google-secret-old",
		HostedDomain:        "old.example.com",
		ServiceAccountJSON:  `{"project_id":"old"}`,
		DelegatedAdminEmail: "admin-old@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertGoogleGroupRole(ctx, "old-team@example.com", Role("ops")); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateOIDCConfig(ctx, OIDCConfig{
		ProviderName: "Old OIDC",
		IssuerURL:    "https://issuer.old.example.com",
		ClientID:     "oidc-client-old",
		ClientSecret: "oidc-secret-old",
		Scopes:       "openid email profile groups old",
		HostedDomain: "old.example.com",
		EmailClaim:   "email",
		GroupsClaim:  "groups",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertOIDCGroupRole(ctx, "old-oidc-group", Role("ops")); err != nil {
		t.Fatal(err)
	}

	exported, err := store.ExportConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.UpdateGoogleConfig(ctx, GoogleConfig{
		ClientID:            "google-client-new",
		ClientSecret:        "google-secret-new",
		HostedDomain:        "new.example.com",
		ServiceAccountJSON:  `{"project_id":"new"}`,
		DelegatedAdminEmail: "admin-new@example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteGoogleGroupRole(ctx, "old-team@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertGoogleGroupRole(ctx, "new-team@example.com", RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateOIDCConfig(ctx, OIDCConfig{
		ProviderName: "New OIDC",
		IssuerURL:    "https://issuer.new.example.com",
		ClientID:     "oidc-client-new",
		ClientSecret: "oidc-secret-new",
		Scopes:       "openid email profile groups new",
		HostedDomain: "new.example.com",
		EmailClaim:   "preferred_username",
		GroupsClaim:  "roles",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteOIDCGroupRole(ctx, "old-oidc-group"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertOIDCGroupRole(ctx, "new-oidc-group", RoleAdmin); err != nil {
		t.Fatal(err)
	}

	if err := store.ImportConfigSnapshot(ctx, exported); err != nil {
		t.Fatal(err)
	}

	googleConfig, err := store.GetGoogleConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if googleConfig != exported.Google.Config {
		t.Fatalf("google config was not overwritten by import: got %#v want %#v", googleConfig, exported.Google.Config)
	}
	googleMappings, err := store.ListGoogleGroupRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(googleMappings) != 1 || googleMappings[0].GroupEmail != "old-team@example.com" || googleMappings[0].Role != Role("ops") {
		t.Fatalf("google mappings were not overwritten by import: %#v", googleMappings)
	}

	oidcConfig, err := store.GetOIDCConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if oidcConfig != exported.OIDC.Config {
		t.Fatalf("oidc config was not overwritten by import: got %#v want %#v", oidcConfig, exported.OIDC.Config)
	}
	oidcMappings, err := store.ListOIDCGroupRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(oidcMappings) != 1 || oidcMappings[0].GroupName != "old-oidc-group" || oidcMappings[0].Role != Role("ops") {
		t.Fatalf("oidc mappings were not overwritten by import: %#v", oidcMappings)
	}
}

func assertAdminRoleHasNoExplicitPermissions(t *testing.T, roles []RoleDef) {
	t.Helper()
	role := findRole(t, roles, string(RoleAdmin))
	if string(role.Permissions) != "{}" {
		t.Fatalf("admin permissions should stay empty because mode=admin grants access: %s", string(role.Permissions))
	}
}

func assertRoleAbsent(t *testing.T, roles []RoleDef, name string) {
	t.Helper()
	for _, role := range roles {
		if role.Name == name {
			t.Fatalf("role %q should not be created by default", name)
		}
	}
}

func findRole(t *testing.T, roles []RoleDef, name string) RoleDef {
	t.Helper()
	for _, role := range roles {
		if role.Name == name {
			return role
		}
	}
	t.Fatalf("role %q not found", name)
	return RoleDef{}
}

func TestConfigSnapshotRejectsUnknownFields(t *testing.T) {
	_, err := DecodeConfigSnapshotBytes([]byte(`
schema_version: 1
initialized: false
roles: []
users: []
google:
  config: {}
  mappings: []
oidc:
  config: {}
  mappings: []
unexpected: true
`))
	if err == nil {
		t.Fatal("expected unknown field decode failure")
	}
	if stage := ConfigImportStage(err); stage != "decode" {
		t.Fatalf("expected decode stage, got %q: %v", stage, err)
	}
}
