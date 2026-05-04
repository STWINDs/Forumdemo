import { api } from './api.js';
import { el, escapeHtml, formatDate } from './utils.js';
import { renderCommentVoteButtons } from './vote.js';

export async function renderComments(postId, containerEl, sort = 'hot') {
  containerEl.innerHTML = '';

  const refresh = (newSort) => renderComments(postId, containerEl, newSort || sort);

  try {
    const comments = await api.get(`/post/${postId}/comments?sort=${sort}`);

    // Sort toggle bar
    const sortBar = el('div', { className: 'sort-toggle' });
    const hotLink = el('a', { href: 'javascript:void(0)' }, '最热');
    const sep = el('span', {}, '|');
    const newLink = el('a', { href: 'javascript:void(0)' }, '最新');
    if (sort === 'hot') { hotLink.className = 'active'; } else { newLink.className = 'active'; }
    hotLink.addEventListener('click', () => refresh('hot'));
    newLink.addEventListener('click', () => refresh('new'));
    sortBar.appendChild(hotLink);
    sortBar.appendChild(sep);
    sortBar.appendChild(newLink);

    containerEl.appendChild(sortBar);
    containerEl.appendChild(el('h2', {}, 'COMMENTS'));

    if (!comments || comments.length === 0) {
      const empty = el('div', { className: 'empty-state', style: 'padding:48px 0;margin:16px 0' });
      empty.appendChild(el('p', {}, 'NOTHING HERE'));
      containerEl.appendChild(empty);
      containerEl.appendChild(el('h3', {}, 'LEAVE A COMMENT'));
      const topForm = el('div');
      buildReplyForm(topForm, postId, 0, () => refresh(sort));
      containerEl.appendChild(topForm);
      return;
    }

    const tree = {};
    const roots = [];
    for (const c of comments) {
      if (c.parent_id === 0) {
        roots.push(c);
      } else {
        if (!tree[c.parent_id]) tree[c.parent_id] = [];
        tree[c.parent_id].push(c);
      }
    }

    function renderComment(comment, depth) {
      const div = el('div', { className: 'comment-item' });
      div.style.marginLeft = `${Math.min(depth * 16, 80)}px`;

      const meta = el('div', { className: 'comment-meta' });
      meta.textContent = `#${comment.author_id}  /  ${formatDate(comment.create_time)}`;
      div.appendChild(meta);
      div.appendChild(el('p', {}, escapeHtml(comment.content)));

      // Mini vote buttons
      const voteRow = el('div', { className: 'vote-group' });
      voteRow.style.margin = '4px 0';
      renderCommentVoteButtons(
        comment.id,
        comment.upvotes || 0,
        comment.downvotes || 0,
        comment.my_vote || 0,
        voteRow
      );
      div.appendChild(voteRow);

      // Reply link
      const replyLink = el('a', { href: 'javascript:void(0)' }, 'REPLY');
      const replyForm = el('div');
      replyForm.style.display = 'none';
      replyForm.style.marginTop = 'var(--space-sm)';
      div.appendChild(replyLink);
      div.appendChild(replyForm);

      replyLink.addEventListener('click', () => {
        if (replyForm.style.display === 'none') {
          replyForm.style.display = 'block';
          replyForm.innerHTML = '';
          buildReplyForm(replyForm, postId, comment.id, () => refresh(sort));
        } else {
          replyForm.style.display = 'none';
          replyForm.innerHTML = '';
        }
      });

      const children = tree[comment.id] || [];
      for (const child of children) {
        div.appendChild(renderComment(child, depth + 1));
      }

      return div;
    }

    for (const root of roots) {
      containerEl.appendChild(renderComment(root, 0));
    }

    containerEl.appendChild(el('h3', {}, 'LEAVE A COMMENT'));
    const topForm = el('div');
    buildReplyForm(topForm, postId, 0, () => refresh(sort));
    containerEl.appendChild(topForm);

  } catch (err) {
    containerEl.appendChild(el('div', { className: 'error-msg' }, `[ERROR] ${escapeHtml(err.message)}`));
  }
}

function buildReplyForm(container, postId, parentId, onCommentAdded) {
  const errorDiv = el('div', { className: 'error-msg' });
  const successDiv = el('div', { className: 'success-msg' });
  const textarea = el('textarea', { rows: '3' });
  const submitBtn = el('button', {}, parentId === 0 ? 'COMMENT' : 'REPLY');

  container.appendChild(textarea);
  container.appendChild(errorDiv);
  container.appendChild(successDiv);
  container.appendChild(submitBtn);

  submitBtn.addEventListener('click', async () => {
    const content = textarea.value.trim();
    if (!content) {
      errorDiv.textContent = '[ERROR] Comment cannot be empty.';
      return;
    }
    errorDiv.textContent = '';
    successDiv.textContent = '';
    try {
      await api.post('/comment', { content, post_id: parseInt(postId), parent_id: parseInt(parentId) });
      successDiv.textContent = '[POSTED]';
      setTimeout(() => onCommentAdded(), 500);
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
    }
  });
}
