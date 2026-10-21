const API_BASE = 'https://deafening-overtime-penknife.ngrok-free.dev';

const STICKER_MAP = {
  warmup_artist: '../assets/stickers/warmup_artist.svg',
  rules_are_rules: '../assets/stickers/rules_are_rules.svg',
  blind_picasso: '../assets/stickers/blind_picasso.svg',
  absolute_menace: '../assets/stickers/absolute_menace.svg',
  what_am_i_looking_at: '../assets/stickers/what_am_i_looking_at.svg',
};

const ALL_TIERS = [
  { name: 'The Warm-Up Artist', rarity: 'Common', min_points: 0, color_hex: '#10b981', sticker_id: 'warmup_artist' },
  { name: 'Rules Are Rules', rarity: 'Uncommon', min_points: 100, color_hex: '#3b82f6', sticker_id: 'rules_are_rules' },
  { name: 'Blind Picasso', rarity: 'Rare', min_points: 300, color_hex: '#a855f7', sticker_id: 'blind_picasso' },
  { name: 'Absolute Menace', rarity: 'Epic', min_points: 750, color_hex: '#ef4444', sticker_id: 'absolute_menace' },
  { name: 'What Am I Looking At?', rarity: 'Legendary', min_points: 1500, color_hex: '#f59e0b', sticker_id: 'what_am_i_looking_at' },
];

document.addEventListener('DOMContentLoaded', () => {
  checkServerHealth();
  initListeners();
  loadUserData();
});

function initListeners() {
  document.getElementById('btn-refresh').addEventListener('click', () => {
    checkServerHealth();
    loadUserData();
  });

  document.getElementById('btn-discord-login').addEventListener('click', () => {
    chrome.tabs.create({ url: `${API_BASE}/api/auth/discord/login` });
  });

  document.getElementById('btn-dev-login').addEventListener('click', handleDevLogin);

  document.getElementById('btn-logout').addEventListener('click', () => {
    chrome.runtime.sendMessage({ type: 'LOGOUT' }, () => {
      showLoggedOutView();
    });
  });
}

async function checkServerHealth() {
  const statusEl = document.getElementById('server-status');
  try {
    const res = await fetch(`${API_BASE}/health`, { signal: AbortSignal.timeout(3000) });
    if (res.ok) {
      statusEl.textContent = 'Server Online';
      statusEl.style.color = '#10b981';
    } else {
      statusEl.textContent = 'Server Error';
      statusEl.style.color = '#ef4444';
    }
  } catch (e) {
    statusEl.textContent = 'Server Offline';
    statusEl.style.color = '#ef4444';
  }
}

async function loadUserData() {
  chrome.runtime.sendMessage({ type: 'GET_AUTH_STATE' }, async (res) => {
    if (!res || !res.token) {
      showLoggedOutView();
      return;
    }

    try {
      const resp = await fetch(`${API_BASE}/api/auth/me`, {
        headers: { Authorization: `Bearer ${res.token}` },
      });

      if (!resp.ok) {
        showLoggedOutView();
        return;
      }

      const data = await resp.json();
      showLoggedInView(data.user, data.history || []);
    } catch (e) {
      if (res.user) {
        showLoggedInView(res.user, []);
      } else {
        showLoggedOutView();
      }
    }
  });
}

async function handleDevLogin() {
  const usernameInput = document.getElementById('input-dev-username');
  const pointsInput = document.getElementById('input-dev-points');

  const username = usernameInput.value.trim() || 'ArtistMaster';
  const points = parseInt(pointsInput.value, 10) || 0;

  chrome.runtime.sendMessage({ type: 'DEV_LOGIN', username, points }, (res) => {
    if (res && res.success) {
      loadUserData();
    } else {
      alert(`Login failed: ${res ? res.error : 'Unknown error'}`);
    }
  });
}

function showLoggedOutView() {
  document.getElementById('auth-logged-out').style.display = 'block';
  document.getElementById('auth-logged-in').style.display = 'none';
}

function showLoggedInView(user, history) {
  document.getElementById('auth-logged-out').style.display = 'none';
  document.getElementById('auth-logged-in').style.display = 'block';

  // User Profile
  document.getElementById('user-name').textContent = user.username;
  document.getElementById('user-points').textContent = user.points;

  const avatarEl = document.getElementById('user-avatar');
  avatarEl.src = user.avatar_url || '../assets/icons/icon.svg';

  const tier = user.tier || ALL_TIERS[0];
  const tierPill = document.getElementById('user-tier-badge');
  tierPill.textContent = `${tier.name} (${tier.rarity})`;
  tierPill.style.background = `${tier.color_hex}22`;
  tierPill.style.borderColor = tier.color_hex;
  tierPill.style.color = tier.color_hex;

  const stickerImg = document.getElementById('user-sticker-badge');
  stickerImg.src = STICKER_MAP[tier.sticker_id] || STICKER_MAP.warmup_artist;
  stickerImg.title = tier.name;

  // Progress Bar calculation
  updateProgressBar(user.points, tier);

  // Render Tiers Showcase
  renderTiersShowcase(user.points, tier);

  // Render History
  renderHistory(history);
}

function updateProgressBar(points, currentTier) {
  let nextTier = null;
  for (let i = 0; i < ALL_TIERS.length; i++) {
    if (ALL_TIERS[i].min_points > points) {
      nextTier = ALL_TIERS[i];
      break;
    }
  }

  const currentLabel = document.getElementById('current-tier-label');
  const nextLabel = document.getElementById('next-tier-label');
  const fillBar = document.getElementById('progress-bar-fill');
  const subText = document.getElementById('progress-pts-left');

  currentLabel.textContent = currentTier.name;

  if (!nextTier) {
    // Max tier reached
    nextLabel.textContent = 'MAX TIER';
    fillBar.style.width = '100%';
    fillBar.style.background = currentTier.color_hex;
    subText.textContent = '🌟 Supreme Doodle Overlord!';
    return;
  }

  nextLabel.textContent = `Next: ${nextTier.name}`;
  const prevMin = currentTier.min_points;
  const range = nextTier.min_points - prevMin;
  const currentProgress = points - prevMin;
  const percent = Math.min(100, Math.max(0, Math.round((currentProgress / range) * 100)));

  fillBar.style.width = `${percent}%`;
  fillBar.style.background = `linear-gradient(90deg, ${currentTier.color_hex}, ${nextTier.color_hex})`;
  subText.textContent = `${nextTier.min_points - points} pts to ${nextTier.name}`;
}

function renderTiersShowcase(userPoints, currentTier) {
  const container = document.getElementById('tiers-showcase');
  container.innerHTML = '';

  ALL_TIERS.forEach((tier) => {
    const isUnlocked = userPoints >= tier.min_points;
    const isCurrent = currentTier.name === tier.name;

    const div = document.createElement('div');
    div.className = `tier-item ${isUnlocked ? 'unlocked' : 'locked'}`;

    div.innerHTML = `
      <img src="${STICKER_MAP[tier.sticker_id]}" class="tier-item-img" alt="${tier.name}">
      <div class="tier-item-info">
        <div class="tier-item-name" style="color: ${tier.color_hex};">${tier.name} (${tier.rarity})</div>
        <div class="tier-item-pts">${tier.min_points} points required</div>
      </div>
      <div class="tier-item-status" style="color: ${isCurrent ? '#fbbf24' : isUnlocked ? '#10b981' : '#64748b'};">
        ${isCurrent ? '★ ACTIVE' : isUnlocked ? '✓ UNLOCKED' : '🔒 LOCKED'}
      </div>
    `;

    container.appendChild(div);
  });
}

function renderHistory(history) {
  const container = document.getElementById('history-container');
  if (!history || history.length === 0) {
    container.innerHTML = `<div class="empty-history">No completed dares recorded yet.</div>`;
    return;
  }

  container.innerHTML = '';
  history.slice(0, 10).forEach((item) => {
    const row = document.createElement('div');
    row.className = 'history-item';
    row.innerHTML = `
      <div class="history-text" title="${escapeHTML(item.dare_text)}">
        <strong>${escapeHTML(item.word || 'Word')}:</strong> ${escapeHTML(item.dare_text)}
      </div>
      <span class="history-reward">+${item.points_given} pts</span>
    `;
    container.appendChild(row);
  });
}

function escapeHTML(str) {
  if (!str) return '';
  return str.replace(/[&<>'"]/g, (tag) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;',
  }[tag] || tag));
}
