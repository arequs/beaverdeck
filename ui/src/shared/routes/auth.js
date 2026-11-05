export const AUTH_API_ROOT = '/api/auth';

export const AUTH_API_ROUTES = {
  providers: `${AUTH_API_ROOT}/providers`,
  bootstrapStatus: `${AUTH_API_ROOT}/bootstrap/status`,
  bootstrapComplete: `${AUTH_API_ROOT}/bootstrap/complete`,
  login: `${AUTH_API_ROOT}/login`,
  logout: `${AUTH_API_ROOT}/logout`,
  googleStart: `${AUTH_API_ROOT}/google/start`,
  oidcStart: `${AUTH_API_ROOT}/oidc/start`,
  entraStart: `${AUTH_API_ROOT}/entra/start`
};
