import { api, setToken, clearToken } from './api.js';
import { el } from './utils.js';

export function renderLoginPage() {
  const main = document.getElementById('main');
  main.innerHTML = `<h1>LOGIN</h1>`;

  const form = el('form');
  const errorDiv = el('div', { className: 'error-msg' });

  form.appendChild(el('label', {}, 'Username'));
  form.appendChild(el('input', { name: 'username', type: 'text', required: '' }));

  form.appendChild(el('label', {}, 'Password'));
  form.appendChild(el('input', { name: 'password', type: 'password', required: '' }));

  form.appendChild(errorDiv);

  form.appendChild(el('button', { type: 'submit' }, 'SIGN IN'));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorDiv.textContent = '';
    const username = form.querySelector('[name="username"]').value.trim();
    const password = form.querySelector('[name="password"]').value;
    if (!username || !password) {
      errorDiv.textContent = '[ERROR] Username and password are required.';
      return;
    }
    try {
      const data = await api.post('/login', { username, password });
      setToken(data.token);
      window.location.hash = '#/';
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
    }
  });

  main.appendChild(form);

  main.appendChild(el('p', { style: 'margin-top:var(--space-lg)' },
    el('a', { href: '#/signup' }, 'Create an account →')
  ));
}

export function renderSignupPage() {
  const main = document.getElementById('main');
  main.innerHTML = `<h1>SIGN UP</h1>`;

  const form = el('form');
  const errorDiv = el('div', { className: 'error-msg' });
  const successDiv = el('div', { className: 'success-msg' });

  form.appendChild(el('label', {}, 'Username'));
  form.appendChild(el('input', { name: 'username', type: 'text', required: '' }));

  form.appendChild(el('label', {}, 'Email'));
  form.appendChild(el('input', { name: 'email', type: 'email', required: '' }));

  form.appendChild(el('label', {}, 'Password'));
  form.appendChild(el('input', { name: 'password', type: 'password', required: '' }));

  form.appendChild(errorDiv);
  form.appendChild(successDiv);

  form.appendChild(el('button', { type: 'submit' }, 'CREATE ACCOUNT'));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorDiv.textContent = '';
    successDiv.textContent = '';
    const username = form.querySelector('[name="username"]').value.trim();
    const email = form.querySelector('[name="email"]').value.trim();
    const password = form.querySelector('[name="password"]').value;
    if (!username || !email || !password) {
      errorDiv.textContent = '[ERROR] All fields are required.';
      return;
    }
    try {
      await api.post('/signup', { username, email, password });
      successDiv.textContent = '[CREATED] Redirecting...';
      setTimeout(() => { window.location.hash = '#/login'; }, 1200);
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
    }
  });

  main.appendChild(form);

  main.appendChild(el('p', { style: 'margin-top:var(--space-lg)' },
    el('a', { href: '#/login' }, 'Already have an account? Sign in →')
  ));
}

export function logout() {
  clearToken();
  window.location.hash = '#/login';
}
