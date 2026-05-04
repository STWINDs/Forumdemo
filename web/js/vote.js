import { api } from './api.js';
import { el } from './utils.js';

// Two-button toggle: click same direction to cancel.
// Active state: .active-up (success green) or .active-down (accent red)
// Count: upvotes only displayed

export async function renderPostVoteButtons(postId, containerEl) {
  containerEl.innerHTML = '';

  const msgDiv = el('div', { className: 'info-msg', style: 'margin-top:var(--space-xs)' });
  const countSpan = el('span', { className: 'vote-count' }, '0');
  const upBtn = el('button', {}, '▲');
  const downBtn = el('button', {}, '▼');

  let myVote = 0;
  let upvotes = 0;

  try {
    const info = await api.get(`/post/${postId}/votes`);
    myVote = info.my_vote;
    upvotes = info.upvotes;
  } catch (e) { /* defaults */ }

  function updateUI() {
    countSpan.textContent = String(upvotes);
    upBtn.className = myVote === 1 ? 'active-up' : '';
    downBtn.className = myVote === -1 ? 'active-down' : '';
  }

  updateUI();

  async function doVote(direction) {
    try {
      const res = await api.post('/vote', {
        post_id: String(postId),
        direction: String(direction)
      });
      // Re-fetch to get accurate counts
      const info = await api.get(`/post/${postId}/votes`);
      myVote = info.my_vote;
      upvotes = info.upvotes;
      updateUI();
      msgDiv.textContent = res.direction === 0 ? '[CLEARED]' : `[${direction === 1 ? 'UPVOTED' : 'DOWNVOTED'}]`;
      msgDiv.className = 'success-msg';
    } catch (err) {
      msgDiv.textContent = `[ERROR] ${err.message}`;
      msgDiv.className = 'error-msg';
    }
  }

  upBtn.addEventListener('click', () => doVote(1));
  downBtn.addEventListener('click', () => doVote(-1));

  containerEl.appendChild(upBtn);
  containerEl.appendChild(downBtn);
  containerEl.appendChild(countSpan);
  containerEl.appendChild(msgDiv);
}

export function renderCommentVoteButtons(commentId, initialUp, initialDown, initialMyVote, containerEl) {
  containerEl.innerHTML = '';

  let myVote = initialMyVote || 0;
  let upvotes = initialUp || 0;

  const countSpan = el('span', { className: 'vote-count' }, String(upvotes));
  const upBtn = el('button', {}, '▲');
  const downBtn = el('button', {}, '▼');

  function updateUI() {
    countSpan.textContent = String(upvotes);
    upBtn.className = myVote === 1 ? 'active-up' : '';
    downBtn.className = myVote === -1 ? 'active-down' : '';
  }

  updateUI();

  async function doVote(direction) {
    try {
      const res = await api.post('/comment-vote', {
        comment_id: String(commentId),
        direction: String(direction)
      });
      const actual = res.direction;
      if (actual === 0) {
        if (myVote === 1) upvotes--;
        myVote = 0;
      } else if (myVote === 0) {
        if (actual === 1) upvotes++;
        myVote = actual;
      } else {
        // Flip: from +1 to -1 or vice versa
        if (myVote === 1) upvotes--;
        if (actual === 1) upvotes++;
        myVote = actual;
      }
      updateUI();
    } catch (err) {
      // silent for mini buttons
    }
  }

  upBtn.addEventListener('click', () => doVote(1));
  downBtn.addEventListener('click', () => doVote(-1));

  containerEl.appendChild(upBtn);
  containerEl.appendChild(downBtn);
  containerEl.appendChild(countSpan);
}
