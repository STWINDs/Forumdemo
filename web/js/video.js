import { api } from './api.js';
import { el } from './utils.js';

export function renderUploadForm() {
  const main = document.getElementById('main');
  main.innerHTML = `<h1>UPLOAD VIDEO</h1>`;

  const form = el('form');
  const errorDiv = el('div', { className: 'error-msg' });
  const successDiv = el('div', { className: 'success-msg' });

  form.appendChild(el('label', {}, 'Title'));
  form.appendChild(el('input', { name: 'title', type: 'text', required: '' }));

  form.appendChild(el('label', {}, 'Video File (.mp4)'));
  form.appendChild(el('input', { name: 'video', type: 'file', accept: 'video/mp4', required: '' }));

  form.appendChild(errorDiv);
  form.appendChild(successDiv);

  form.appendChild(el('button', { type: 'submit' }, 'UPLOAD'));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorDiv.textContent = '';
    successDiv.textContent = '';

    const title = form.querySelector('[name="title"]').value.trim();
    const fileInput = form.querySelector('[name="video"]');
    const file = fileInput.files[0];

    if (!title || !file) {
      errorDiv.textContent = '[ERROR] Title and file are required.';
      return;
    }

    const formData = new FormData();
    formData.append('title', title);
    formData.append('video', file);

    try {
      await api.upload('/video/upload', formData);
      successDiv.textContent = '[UPLOADED]';
      form.reset();
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
    }
  });

  main.appendChild(form);
}
