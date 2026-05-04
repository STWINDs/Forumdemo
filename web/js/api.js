const BASE = '/api/v1';

function token() {
  return localStorage.getItem('forum_token');
}

export function setToken(t) {
  localStorage.setItem('forum_token', t);
}

export function clearToken() {
  localStorage.removeItem('forum_token');
}

export function hasToken() {
  return !!localStorage.getItem('forum_token');
}

export function getCurrentUserID() {
  const t = localStorage.getItem('forum_token');
  if (!t) return null;
  try {
    const payload = JSON.parse(atob(t.split('.')[1]));
    return payload.user_id;
  } catch (e) {
    return null;
  }
}

async function request(method, path, body) {
  const headers = {};
  const t = token();
  if (t) headers['Authorization'] = 'Bearer ' + t;

  let fetchBody;
  if (body && !(body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
    fetchBody = JSON.stringify(body);
  } else {
    fetchBody = body;
  }

  const res = await fetch(BASE + path, { method, headers, body: fetchBody });

  if (res.status === 401) {
    clearToken();
    window.location.hash = '#/login';
    throw new Error('Unauthorized');
  }
  if (res.status === 429) {
    throw new Error('That\'s too frequent. Wait.');
  }

  const data = await res.json().catch(() => ({}));

  if (!res.ok) {
    throw new Error(data.msg || 'Request failed');
  }

  return data;
}

export const api = {
  get:    (path)            => request('GET', path),
  post:   (path, body)      => request('POST', path, body),
  put:    (path, body)      => request('PUT', path, body),
  del:    (path)            => request('DELETE', path),
  upload: (path, formData)  => request('POST', path, formData),
};
