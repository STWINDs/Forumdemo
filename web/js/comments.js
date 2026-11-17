import { api, getCurrentUserID } from './api.js';
import { el, escapeHtml, formatDate } from './utils.js';
import { renderCommentVoteButtons } from './vote.js';

export async function renderComments(postId, containerEl, sort = 'hot') {
  containerEl.innerHTML = '';

  const refresh = (newSort) => renderComments(postId, containerEl, newSort || sort);
	const pendingPanel = el('div');
	containerEl.appendChild(pendingPanel);
	const storageKey = `forum_pending_comments:${getCurrentUserID()}:${postId}`;
	let pending = [];
	try { pending = JSON.parse(sessionStorage.getItem(storageKey) || '[]'); } catch (_) { /* storage unavailable */ }
	const save = () => { try { sessionStorage.setItem(storageKey, JSON.stringify(pending)); } catch (_) { /* retain in memory */ } };
	const sameSubmission = (a, b) => a.client_comment_id === b.client_comment_id && a.fingerprint === b.fingerprint;
	function showPending(comment) {
	  const card = el('div', { className: 'comment-item' });
	  card.appendChild(el('p', {}, comment.content));
	  const state = el('span', {}, '处理中 · 仅自己可见');
	  card.appendChild(state);
	  pendingPanel.appendChild(card);
	  function showFailed() {
	    state.textContent = '发布失败，可重试';
	    const retry = el('button', {type: 'button'}, '重试发布');
	    retry.addEventListener('click', async () => {
	      retry.disabled = true;
	      comment.retry_id ||= newCommentRequestID();
	      save();
	      try {
	        const receipt = await api.post('/comment', {
	          post_id: comment.post_id, parent_id: comment.parent_id,
	          content: comment.content, client_comment_id: comment.retry_id
	        });
	        onAccepted(receipt);
	        pending = pending.filter(c => !sameSubmission(c, comment));
	        save();
	        card.remove();
	      } catch (_) {
	        state.textContent = '暂未确认重试结果，可再次重试';
	      } finally {
	        retry.disabled = false;
	      }
	    });
	    card.appendChild(retry);
	  }
	  if (comment.failed) { showFailed(); return; }
	  let attempts = 0;
	  const poll = async () => {
	    if (!card.isConnected) return;
	    attempts += 1;
	    try {
	      const query = comment.fingerprint ? `?fingerprint=${encodeURIComponent(comment.fingerprint)}` : '';
	      const receipt = await api.get(`/comments/submissions/${encodeURIComponent(comment.client_comment_id)}${query}`);
	      if (!card.isConnected) return;
	      if (receipt.state === 'failed') {
	        comment.failed = true;
	        save();
	        showFailed();
	        return;
	      }
	      if (receipt.state !== 'processing') {
	        state.textContent = receipt.state === 'published' ? '已发布' : '该评论当前不可公开展示';
	        pending = pending.filter(c => !sameSubmission(c, comment));
	        save();
	        if (receipt.state === 'published') {
	          const link = el('button', {}, '刷新评论列表');
	          link.addEventListener('click', () => refresh(sort));
	          card.appendChild(link);
	        }
	        return;
	      }
	      if (attempts >= 10) state.textContent = '已受理，处理延迟 · 请勿重复发送';
	    } catch (_) {
	      state.textContent = '已受理，暂时无法确认进度 · 恢复后自动查询';
	    }
	    if (card.isConnected) setTimeout(poll, Math.min(30000, 2000 + attempts * 1000));
	  };
	  setTimeout(poll, 2000);
	}
	const onAccepted = (receipt) => {
	  const entry = {...receipt.comment, fingerprint: receipt.fingerprint};
	  if (!pending.some(c => sameSubmission(c, entry))) {
	    pending.push(entry); save(); showPending(entry);
	  }
	};
	for (const c of pending) showPending(c);
	// Keep submission available even when the public list cannot be queried.
	containerEl.appendChild(el('h3', {}, 'LEAVE A COMMENT'));
	const topForm = el('div');
	buildReplyForm(topForm, postId, 0, onAccepted);
	containerEl.appendChild(topForm);

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
          buildReplyForm(replyForm, postId, comment.id, onAccepted);
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

  } catch (err) {
    containerEl.appendChild(el('div', { className: 'error-msg' }, `[ERROR] ${escapeHtml(err.message)}`));
  }
}

function newCommentRequestID() {
  return crypto.randomUUID ? crypto.randomUUID() : Array.from(crypto.getRandomValues(new Uint8Array(16)), n => n.toString(16).padStart(2, '0')).join('');
}

function buildReplyForm(container, postId, parentId, onCommentAdded) {
	let requestID = null;
	let requestContent = null;
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
	if (requestContent !== content || !requestID) {
	  requestID = newCommentRequestID();
	  requestContent = content;
	}
	submitBtn.disabled = true;
    try {
	  const receipt = await api.post('/comment', { content, post_id: parseInt(postId), parent_id: parseInt(parentId), client_comment_id: requestID });
	  onCommentAdded(receipt);
	  successDiv.textContent = '已受理 · 正在处理';
	  textarea.value = ''; requestID = null; requestContent = null;
    } catch (err) {
      errorDiv.textContent = `[ERROR] ${err.message}`;
	} finally {
	  submitBtn.disabled = false;
    }
  });
}
