import { hasToken } from './api.js';
import { renderLoginPage, renderSignupPage, logout } from './auth.js';
import { renderPostList, renderPostDetail, renderCreatePostForm } from './posts.js';

const publicRoutes = ['/login', '/signup'];

function renderHeader() {
  const header = document.getElementById('header');
  const nav = document.createElement('nav');

  if (hasToken()) {
    nav.appendChild(link('#/', '[ HOME ]'));
    nav.appendChild(link('#/new-post', 'NEW POST'));
    nav.appendChild(text('  '));
    nav.appendChild(link('#/logout', 'LOGOUT'));
  } else {
    nav.appendChild(link('#/', '[ FORUM ]'));
    nav.appendChild(link('#/login', 'LOGIN'));
    nav.appendChild(link('#/signup', 'SIGN UP'));
  }

  header.innerHTML = '';
  header.appendChild(nav);
}

function link(href, label) {
  const a = document.createElement('a');
  a.href = href;
  a.textContent = label;
  return a;
}

function text(s) {
  const span = document.createElement('span');
  span.textContent = s;
  return span;
}

function router() {
  renderHeader();

  let hash = window.location.hash.slice(1) || '/';
  if (hash.length > 1 && hash.endsWith('/')) hash = hash.slice(0, -1);

  let route = hash;
  let params = {};

  const postMatch = hash.match(/^\/post\/(\d+)$/);
  if (postMatch) {
    route = '/post/:id';
    params.id = parseInt(postMatch[1]);
  }

  if (!publicRoutes.includes(route) && route !== '/logout' && !hasToken()) {
    window.location.hash = '#/login';
    return;
  }
  if (publicRoutes.includes(route) && hasToken()) {
    window.location.hash = '#/';
    return;
  }

  const main = document.getElementById('main');

  switch (route) {
    case '/':
    case '':
      renderPostList(1);
      break;
    case '/login':
      renderLoginPage();
      break;
    case '/signup':
      renderSignupPage();
      break;
    case '/post/:id':
      renderPostDetail(params.id);
      break;
    case '/new-post':
      renderCreatePostForm();
      break;
    case '/logout':
      logout();
      break;
    default:
      const pageMatch = hash.match(/^\/\?page=(\d+)$/);
      if (pageMatch) {
        renderPostList(parseInt(pageMatch[1]));
        return;
      }
      main.innerHTML = `<div class="empty-state"><p>NOTHING HERE</p></div>`;
  }
}

window.addEventListener('hashchange', router);
window.addEventListener('DOMContentLoaded', router);
