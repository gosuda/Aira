import createClient from 'openapi-fetch';

// Base URL is empty in production (same-origin), proxied in dev
const baseUrl = '';

export const api = createClient({ baseUrl });
