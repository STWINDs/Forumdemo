import { api, getCurrentUserID } from './api.js';
import { el, escapeHtml, formatDate } from './utils.js';
import { renderComments } from './comments.js';
import { renderPostVoteButtons } from './vote.js';

const PREVIEW_LEN = 30;

export async function renderPostList(page = 1) {
  const main = document.getElementById('main');
  main.innerHTML = `<h1>POSTS</h1><div class="loading">[ LOADING ]</div>`;

  try {
    const summaries = await api.get(`/posts?page=${page}&size=10`);
    main.innerHTML = `<h1>POSTS</h1>`;

    if (!summaries || summaries.length === 0) {
      const empty = el('div', { className: 'empty-state' });
      empty.appendChild(el('p', {}, 'JUST QUIET'));
      main.appendChild(empty);
      return;
    }

    for (const s of summaries) {
      const p = s.post;
      const card = el('div', { className: 'post-card' });

      // Title
      card.appendChild(el('h3', {}, el('a', { href: `#/post/${p.id}` }, escapeHtml(p.title))));

      // Meta: author + time
      const meta = el('div', { className: 'card-meta' });
      meta.textContent = `#${p.author_id}  /  ${formatDate(p.create_time)}`;
      card.appendChild(meta);

      // Video embed
      if (p.post_type === 3 && p.video_url) {
        const vid = el('video', { controls: '', preload: 'metadata' });
        vid.src = p.video_url;
        card.appendChild(vid);
      }

      // Link preview
      if (p.post_type === 2) {
        const url = extractURL(p.content);
        if (url) {
          card.appendChild(el('div', { className: 'card-link' },
            el('a', { href: url, target: '_blank', rel: 'noopener' }, url)
          ));
        }
      }

      // Content preview
      const preview = el('div', { className: 'card-preview' });
      const fullText = p.content || '';
      const chars = [...fullText];
      const truncated = chars.slice(0, PREVIEW_LEN).join('');
      const isLong = chars.length > PREVIEW_LEN;

      preview.textContent = truncated + (isLong ? '...' : '');
      card.appendChild(preview);

      if (isLong) {
        const expandBtn = el('button', { className: 'card-expand' }, '[EXPAND ▼]');
        let expanded = false;
        expandBtn.addEventListener('click', () => {
          expanded = !expanded;
          preview.textContent = expanded ? fullText : truncated + '...';
          expandBtn.textContent = expanded ? '[COLLAPSE ▲]' : '[EXPAND ▼]';
        });
        card.appendChild(expandBtn);
      }

      // Stats row: votes + comments
      const stats = el('div', { className: 'card-stats' });
      stats.appendChild(el('span', {}, `▲ ${s.upvotes}`));
      stats.appendChild(el('span', {}, `💬 ${s.comment_count}`));
      card.appendChild(stats);

      // Top comment
      if (s.top_comment) {
        const tc = s.top_comment;
        const tcDiv = el('div', { className: 'card-top-comment' });
        tcDiv.appendChild(el('p', {}, escapeHtml(tc.content)));
        const tcMeta = el('div', { className: 'tc-author' });
        tcMeta.textContent = `#${tc.author_id}  ▲ ${tc.upvotes}`;
        tcDiv.appendChild(tcMeta);
        card.appendChild(tcDiv);
      }

      main.appendChild(card);
    }

    // Pagination
    const pag = el('div', { className: 'pagination' });
    if (page > 1) {
      pag.appendChild(el('a', { href: `#/?page=${page - 1}` }, '← PREV'));
    }
    pag.appendChild(document.createTextNode(` PAGE ${page} `));
    if (summaries.length === 10) {
      pag.appendChild(el('a', { href: `#/?page=${page + 1}` }, 'NEXT →'));
    }
    main.appendChild(pag);

  } catch (err) {
    main.innerHTML = `<h1>POSTS</h1>`;
    main.appendChild(el('div', { className: 'error-msg' }, `[ERROR] ${escapeHtml(err.message)}`));
  }
}

export async function renderPostDetail(id) {
  const main = document.getElementById('main');
  main.innerHTML = `<div class="loading">[ LOADING ]</div>`;

  try {
    const post = await api.get(`/post/${id}`);
    main.innerHTML = '';

    main.appendChild(el('h1', {}, escapeHtml(post.title)));

    const meta = el('div', { className: 'post-meta' });
    meta.textContent = `BY #${post.author_id}  /  ${formatDate(post.create_time)}`;
    main.appendChild(meta);

    // Video embed
    if (post.post_type === 3 && post.video_url) {
      const vid = el('video', { controls: '', preload: 'metadata', style: 'max-width:100%;max-height:480px;margin:16px 0' });
      vid.src = post.video_url;
      main.appendChild(vid);
    }

    // Link
    if (post.post_type === 2) {
      const url = extractURL(post.content);
      if (url) {
        main.appendChild(el('p', {},
          el('a', { href: url, target: '_blank', rel: 'noopener' }, url)
        ));
      }
    }

    // Owner actions
    const currentUID = getCurrentUserID();
    if (currentUID && post.author_id === currentUID) {
      const actions = el('div', { style: 'margin:var(--space-md) 0' });
      const editBtn = el('button', {}, 'EDIT');
      const delBtn = el('button', {}, 'DELETE');
      actions.appendChild(editBtn);
      actions.appendChild(delBtn);

      editBtn.addEventListener('click', () => showEditForm(post));
      delBtn.addEventListener('click', async () => {
        if (!confirm('Delete this post?')) return;
        try {
          await api.del(`/post/${id}`);
          window.location.hash = '#/';
        } catch (err) {
          alert(err.message);
        }
      });

      main.appendChild(actions);
    }

    main.appendChild(el('p', {}, escapeHtml(post.content)));

    const voteContainer = el('div', { className: 'vote-group' });
    main.appendChild(voteContainer);
    renderPostVoteButtons(id, voteContainer);

    const commentsContainer = el('div', { id: 'comments-container' });
    main.appendChild(commentsContainer);
    await renderComments(id, commentsContainer);

  } catch (err) {
    main.innerHTML = '';
    main.appendChild(el('div', { className: 'error-msg' }, `[ERROR] ${escapeHtml(err.message)}`));
  }
}

export function renderCreatePostForm() {
  const main = document.getElementById('main');
  main.innerHTML = `<h1>NEW POST</h1>`;

  let postType = 1; // 1:text 2:link 3:video

  const form = el('form', { enctype: 'multipart/form-data' });
  const errorDiv = el('div', { className: 'error-msg' });
  const successDiv = el('div', { className: 'success-msg' });

  // Type selector (segmented)
  form.appendChild(el('label', {}, 'Type'));
  const typeToggle = el('div', { className: 'type-toggle' });
  const textBtn = el('button', { type: 'button', className: 'active' }, 'TEXT');
  const linkBtn = el('button', { type: 'button' }, 'LINK');
  const videoBtn = el('button', { type: 'button' }, 'VIDEO');
  typeToggle.appendChild(textBtn);
  typeToggle.appendChild(linkBtn);
  typeToggle.appendChild(videoBtn);
  form.appendChild(typeToggle);

  form.appendChild(el('label', {}, 'Title'));
  form.appendChild(el('input', { name: 'title', type: 'text', required: '' }));

  const contentLabel = el('label', {}, 'Content');
  form.appendChild(contentLabel);
  const textarea = el('textarea', { name: 'content', required: '' });
  form.appendChild(textarea);

  // Video file input (hidden unless type=3)
  const videoInput = el('input', { name: 'video', type: 'file', accept: 'video/mp4' });
  videoInput.style.display = 'none';
  form.appendChild(videoInput);

  form.appendChild(errorDiv);
  form.appendChild(successDiv);
  form.appendChild(el('button', { type: 'submit' }, 'POST'));

  function updateType(t) {
    postType = t;
    [textBtn, linkBtn, videoBtn].forEach(b => b.className = '');
    if (t === 1) { textBtn.className = 'active'; videoInput.style.display = 'none'; }
    if (t === 2) { linkBtn.className = 'active'; videoInput.style.display = 'none'; }
    if (t === 3) { videoBtn.className = 'active'; videoInput.style.display = 'block'; }
  }

  textBtn.addEventListener('click', () => updateType(1));
  linkBtn.addEventListener('click', () => updateType(2));
  videoBtn.addEventListener('click', () => updateType(3));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorDiv.textContent = '';
    successDiv.textContent = '';

    const title = form.querySelector('[name="title"]').value.trim();
    const content = form.querySelector('[name="content"]').value.trim();
    if (!title || !content) {
      errorDiv.textContent = '[ERROR] Title and content are required.';
      return;
    }

    try {
      if (postType === 3) {
        const fileInput = form.querySelector('[name="video"]');
        const file = fileInput.files[0];
        if (!file) { errorDiv.textContent = '[ERROR] Video file is required.'; return; }
        const fd = new FormData();
        fd.append('title', title);
        fd.append('content', content);
        fd.append('post_type', '3');
        fd.append('community_id', '1');
        fd.append('video', file);
        await api.upload('/post', fd);
      } else {
        await api.post('/post', { title, content, post_type: postType, community_id: 1 });
      }
      successDiv.textContent = '[POSTED]';
      setTimeout(() => { window.location.hash = '#/'; }, 600);
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
    }
  });

  main.appendChild(form);
}

function showEditForm(post) {
  const main = document.getElementById('main');
  main.innerHTML = `<h1>EDIT POST</h1>`;

  const form = el('form');
  const errorDiv = el('div', { className: 'error-msg' });
  const successDiv = el('div', { className: 'success-msg' });

  form.appendChild(el('label', {}, 'Title'));
  const titleInput = el('input', { name: 'title', type: 'text', value: post.title, required: '' });
  form.appendChild(titleInput);

  form.appendChild(el('label', {}, 'Content'));
  const textarea = el('textarea', { name: 'content', required: '' });
  textarea.value = post.content;
  form.appendChild(textarea);

  form.appendChild(errorDiv);
  form.appendChild(successDiv);
  form.appendChild(el('button', { type: 'submit' }, 'SAVE'));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    errorDiv.textContent = '';
    successDiv.textContent = '';
    const title = titleInput.value.trim();
    const content = textarea.value.trim();
    if (!title || !content) {
      errorDiv.textContent = '[ERROR] Title and content are required.';
      return;
    }
    try {
      await api.put(`/post/${post.id}`, { title, content, post_type: post.post_type || 1 });
      successDiv.textContent = '[SAVED]';
      setTimeout(() => { window.location.reload(); }, 500);
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
    }
  });

  main.appendChild(form);
}

function extractURL(text) {
  const m = text.match(/(https?:\/\/[^\s<>"]+)/i);
  return m ? m[1] : null;
}
