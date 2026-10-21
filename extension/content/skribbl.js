/**
 * Skribbl Dare — Content Script for Skribbl.io
 * Injects Dare selection cards, Home screen Login & Profile widget,
 * "Completed Dare" reward buttons, and Tier Stickers/Badges into the player list.
 */

(function () {
  'use strict';

  console.log('🎨 [Skribbl Dare] Content script initialized!');

  const API_BASE = 'https://deafening-overtime-penknife.ngrok-free.dev';

  // State
  let currentUser = null;
  let currentToken = null;
  let activeDare = null;
  let chosenWord = '';
  let dareClaimedThisRound = false;
  let roomPlayers = new Map(); // nickname -> PlayerBadgeInfo
  let roomCode = getRoomCode();
  let wsConnection = null;
  let isPromptingDare = false;

  // Tier sticker path map
  const STICKER_URLS = {
    warmup_artist: chrome.runtime.getURL('assets/stickers/warmup_artist.svg'),
    rules_are_rules: chrome.runtime.getURL('assets/stickers/rules_are_rules.svg'),
    blind_picasso: chrome.runtime.getURL('assets/stickers/blind_picasso.svg'),
    absolute_menace: chrome.runtime.getURL('assets/stickers/absolute_menace.svg'),
    what_am_i_looking_at: chrome.runtime.getURL('assets/stickers/what_am_i_looking_at.svg'),
  };

  // --- 1. Initialization ---
  loadAuth();
  startRoomSync();
  observeGameDOM();
  injectCompletedDareButton();
  injectTopbarWidget();
  injectHomeLoginWidget();

  // Listen for storage changes across tabs or extension popup
  chrome.storage.onChanged.addListener((changes, area) => {
    if (area === 'local') {
      if (changes.skribbl_dare_jwt || changes.skribbl_dare_user) {
        chrome.storage.local.get(['skribbl_dare_jwt', 'skribbl_dare_user'], (res) => {
          currentToken = res.skribbl_dare_jwt || null;
          currentUser = res.skribbl_dare_user || null;
          updateAllUI();
          if (currentToken) connectWebSocket();
        });
      }
    }
  });

  function getRoomCode() {
    const path = window.location.pathname;
    const search = window.location.search;
    if (search && search.length > 1) {
      return search.replace('?', '');
    }
    const match = path.match(/\/r\/([a-zA-Z0-9_-]+)/);
    if (match) return match[1];

    const inviteInput = document.querySelector('#input-invite, .input-invite');
    if (inviteInput && inviteInput.value) {
      const parts = inviteInput.value.split('?');
      if (parts[1]) return parts[1];
    }
    return 'public_room';
  }

  function getMyInGameNickname() {
    const nameInput = document.querySelector('.input-name, #input-name');
    if (nameInput && nameInput.value) {
      return nameInput.value.trim();
    }
    const mePlayer = document.querySelector('#game-players .player.me .player-name, #game-players .player .player-name.me, #game-players .player.me .name');
    if (mePlayer) return mePlayer.textContent.trim();
    return currentUser ? currentUser.username : 'Artist';
  }

  function loadAuth() {
    chrome.runtime.sendMessage({ type: 'GET_AUTH_STATE' }, (res) => {
      if (res && res.token) {
        currentToken = res.token;
        currentUser = res.user;
        console.log('⚡ [Skribbl Dare] Authenticated as:', currentUser.username, 'Tier:', currentUser.tier.name);
        connectWebSocket();
        updateAllUI();
      }
    });

    // Listen for auth message from Discord OAuth callback popup
    window.addEventListener('message', (event) => {
      if (event.data && event.data.type === 'SKRIBBL_DARE_AUTH_SUCCESS') {
        currentToken = event.data.token;
        currentUser = event.data.user;
        chrome.storage.local.set({
          skribbl_dare_jwt: currentToken,
          skribbl_dare_user: currentUser
        });
        showToast(`🎉 Connected as ${currentUser.username}! Tier: ${currentUser.tier.name}`);
        connectWebSocket();
        updateAllUI();
      }
    });
  }

  function updateAllUI() {
    injectHomeLoginWidget();
    injectTopbarWidget();
    updatePlayerBadges();
  }

  // --- 2. Home Page Skribbl Dare Widget ---
  function injectHomeLoginWidget() {
    const homePanel = document.querySelector('#home .panel');
    if (!homePanel) return;

    let widget = document.querySelector('#skribbl-dare-home-card');
    if (!widget) {
      widget = document.createElement('div');
      widget.id = 'skribbl-dare-home-card';
      widget.className = 'skribbl-dare-home-card';
      const container = homePanel.querySelector('.container-name-lang') || homePanel.firstChild;
      homePanel.insertBefore(widget, container);
    }

    if (!currentUser || !currentToken) {
      widget.innerHTML = `
        <div class="skribbl-dare-home-header">
          <div class="skribbl-dare-home-title">
            <span>🎲</span>
            <span>Skribbl Dare</span>
          </div>
          <span style="font-size: 10px; color: #10b981; font-weight: 700;">🟢 Online</span>
        </div>
        <div class="skribbl-dare-home-desc">
          Get crazy drawing dares, level up tiers, and flex stickers & badges in-game!
        </div>
        <div class="skribbl-dare-home-actions">
          <button id="skribbl-dare-home-login-btn" class="skribbl-dare-btn-discord">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.929 1.793 8.18 1.793 12.061 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.893.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.028zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
            </svg>
            Connect Discord
          </button>
          <button id="skribbl-dare-home-dev-btn" class="skribbl-dare-btn-dev" title="Quick testing without Discord API credentials">
            ⚡ Quick Dev Login
          </button>
        </div>
      `;

      widget.querySelector('#skribbl-dare-home-login-btn').addEventListener('click', openDiscordLogin);
      widget.querySelector('#skribbl-dare-home-dev-btn').addEventListener('click', triggerQuickDevLogin);
    } else {
      const stickerSrc = STICKER_URLS[currentUser.tier.sticker_id] || STICKER_URLS.warmup_artist;
      const avatarSrc = currentUser.avatar_url || 'https://cdn.discordapp.com/embed/avatars/0.png';

      widget.innerHTML = `
        <div class="skribbl-dare-profile-row">
          <div class="skribbl-dare-profile-info">
            <img class="skribbl-dare-profile-avatar" src="${avatarSrc}" onerror="this.src='https://cdn.discordapp.com/embed/avatars/0.png'">
            <div>
              <div class="skribbl-dare-profile-name">${escapeHTML(currentUser.username)}</div>
              <div class="skribbl-dare-profile-tier" style="border-color: ${currentUser.tier.color_hex}; color: ${currentUser.tier.color_hex}; background: ${currentUser.tier.color_hex}18;">
                <img src="${stickerSrc}" style="width: 14px; height: 14px;">
                ${escapeHTML(currentUser.tier.name)}
              </div>
            </div>
          </div>
          <div style="text-align: right;">
            <div class="skribbl-dare-profile-points">🔥 ${currentUser.points} pts</div>
            <button id="skribbl-dare-home-logout-btn" class="skribbl-dare-btn-logout-small">Logout</button>
          </div>
        </div>
      `;

      widget.querySelector('#skribbl-dare-home-logout-btn').addEventListener('click', handleLogout);
    }
  }

  // --- 3. Persistent Topbar Widget ---
  function injectTopbarWidget() {
    let topbar = document.querySelector('#skribbl-dare-topbar-widget');
    if (!topbar) {
      topbar = document.createElement('div');
      topbar.id = 'skribbl-dare-topbar-widget';
      topbar.className = 'skribbl-dare-topbar-widget';
      document.body.appendChild(topbar);
    }

    if (currentUser && currentToken) {
      const stickerSrc = STICKER_URLS[currentUser.tier.sticker_id] || STICKER_URLS.warmup_artist;
      topbar.innerHTML = `
        <img class="skribbl-dare-topbar-sticker" src="${stickerSrc}" alt="Tier">
        <span>${escapeHTML(currentUser.username)}</span>
        <span style="color: ${currentUser.tier.color_hex}; font-size: 10px;">${escapeHTML(currentUser.tier.name)}</span>
        <span style="color: #f59e0b;">🔥 ${currentUser.points}</span>
      `;
      topbar.title = 'Click to open Skribbl Dare profile';
      topbar.onclick = () => {
        showToast(`👤 ${currentUser.username} | Tier: ${currentUser.tier.name} (${currentUser.tier.rarity}) | ${currentUser.points} pts`);
      };
    } else {
      topbar.innerHTML = `
        <span style="color: #818cf8;">⚡ Skribbl Dare:</span>
        <span style="color: #ffffff; text-decoration: underline;">Login</span>
      `;
      topbar.title = 'Click to login with Discord or Dev mode';
      topbar.onclick = openDiscordLogin;
    }
  }

  function openDiscordLogin() {
    const width = 520;
    const height = 750;
    const left = window.screenX + (window.outerWidth - width) / 2;
    const top = window.screenY + (window.outerHeight - height) / 2;
    window.open(
      `${API_BASE}/api/auth/discord/login`,
      'SkribblDareDiscordAuth',
      `width=${width},height=${height},left=${left},top=${top},status=no,toolbar=no,menubar=no`
    );
  }

  function triggerQuickDevLogin() {
    const defaultNick = getMyInGameNickname() || 'ArtistPro';
    const username = prompt('Enter a test username for instant Dev Login:', defaultNick);
    if (!username) return;

    chrome.runtime.sendMessage({
      type: 'DEV_LOGIN',
      username: username.trim(),
      points: 120
    }, (res) => {
      if (res && res.success) {
        currentToken = res.data.token;
        currentUser = res.data.user;
        showToast(`⚡ Logged in as ${currentUser.username}! Tier: ${currentUser.tier.name}`);
        connectWebSocket();
        updateAllUI();
      } else {
        showToast(`❌ Dev Login error: ${res ? res.error : 'Unknown'}`);
      }
    });
  }

  function handleLogout() {
    chrome.runtime.sendMessage({ type: 'LOGOUT' }, () => {
      currentToken = null;
      currentUser = null;
      showToast('👋 Logged out of Skribbl Dare');
      updateAllUI();
    });
  }

  // --- 4. WebSocket & Room Sync ---
  function connectWebSocket() {
    roomCode = getRoomCode();
    const myNick = getMyInGameNickname();
    const wsBase = API_BASE.replace(/^http/, 'ws');
    const wsUrl = `${wsBase}/ws?token=${encodeURIComponent(currentToken || '')}&room_code=${encodeURIComponent(roomCode)}&nickname=${encodeURIComponent(myNick)}`;

    try {
      if (wsConnection) wsConnection.close();
      wsConnection = new WebSocket(wsUrl);

      wsConnection.onopen = () => {
        console.log('🔌 [Skribbl Dare] Connected to room sync WebSocket:', roomCode);
      };

      wsConnection.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          handleWSEvent(msg);
        } catch (e) {
          console.error('[WS Parse Error]', e);
        }
      };

      wsConnection.onclose = () => {
        setTimeout(() => {
          if (currentToken) connectWebSocket();
        }, 5000);
      };
    } catch (e) {
      console.warn('[WS Connect Error]', e);
    }
  }

  function handleWSEvent(msg) {
    if (msg.type === 'room_players_sync' && msg.payload) {
      const players = typeof msg.payload === 'string' ? JSON.parse(msg.payload) : msg.payload;
      roomPlayers.clear();
      players.forEach(p => {
        if (p.ingame_nickname) roomPlayers.set(p.ingame_nickname.toLowerCase(), p);
        if (p.username) roomPlayers.set(p.username.toLowerCase(), p);
      });
      updatePlayerBadges();
    } else if (msg.type === 'dare_completed') {
      const data = typeof msg.payload === 'string' ? JSON.parse(msg.payload) : msg.payload;
      showToast(`🎯 ${data.nickname || data.user} completed a dare (+${data.points_earned} pts)!`);
      if (data.tier_upgraded) {
        showToast(`🌟 ${data.nickname || data.user} unlocked tier: ${data.tier.name}!`, 5000);
      }
      playCelebrationSound();
      triggerConfetti();
    }
  }

  function startRoomSync() {
    setInterval(async () => {
      const currentCode = getRoomCode();
      if (currentCode !== roomCode) {
        roomCode = currentCode;
        if (currentToken) connectWebSocket();
      }

      try {
        const resp = await fetch(`${API_BASE}/api/rooms/${roomCode}/players`, {
          headers: { 'ngrok-skip-browser-warning': 'true' }
        });
        if (resp.ok) {
          const data = await resp.json();
          if (data.players) {
            data.players.forEach(p => {
              if (p.ingame_nickname) roomPlayers.set(p.ingame_nickname.toLowerCase(), p);
              if (p.username) roomPlayers.set(p.username.toLowerCase(), p);
            });
            updatePlayerBadges();
          }
        }
      } catch (e) {
        // Backend loading
      }
    }, 4000);
  }

  // --- 5. DOM Observer (Word Picker, Badges, Buttons) ---
  function observeGameDOM() {
    const observer = new MutationObserver(() => {
      checkWordChoiceModal();
      injectCompletedDareButton();
      injectHomeLoginWidget();
      updatePlayerBadges();
    });

    observer.observe(document.body, { childList: true, subtree: true, attributes: true });
  }

  // --- 6. Word Selection Dare Injection ---
  async function checkWordChoiceModal() {
    // In skribbl.io, words appear inside .overlay-content .words (or #game-canvas .overlay-content .words)
    const wordsContainer = document.querySelector('#game-canvas .overlay-content .words, .overlay-content .words, .words');
    if (!wordsContainer) return;

    const wordElements = wordsContainer.querySelectorAll('.word');
    const isVisible = wordElements.length > 0 &&
      (wordsContainer.classList.contains('show') ||
       wordsContainer.offsetParent !== null ||
       window.getComputedStyle(wordsContainer).display !== 'none');

    if (isVisible) {
      if (isPromptingDare || document.querySelector('#skribbl-dare-prompt-card')) {
        return;
      }

      isPromptingDare = true;
      dareClaimedThisRound = false;

      try {
        const dare = await fetchDare();
        activeDare = dare;
        renderDareSelectionCard(wordsContainer, dare);
      } catch (e) {
        console.error('Failed to load dare:', e);
      } finally {
        isPromptingDare = false;
      }
    } else {
      const promptCard = document.querySelector('#skribbl-dare-prompt-card');
      if (promptCard) promptCard.remove();
      isPromptingDare = false;
    }
  }

  async function fetchDare() {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ type: 'FETCH_RANDOM_DARE' }, (res) => {
        if (res && res.success && res.dare) {
          resolve(res.dare);
        } else {
          // Fallback dare in case network delay occurs
          resolve({
            id: 'fb-blind-contour',
            text: 'Draw using only one single continuous line without lifting the mouse!',
            category: 'Drawing Constraint',
            difficulty: 'Uncommon',
            points: 50
          });
        }
      });
    });
  }

  function renderDareSelectionCard(wordsContainer, dare) {
    const existing = document.querySelector('#skribbl-dare-prompt-card');
    if (existing) existing.remove();

    const rarityColors = {
      Common: '#10b981',
      Uncommon: '#3b82f6',
      Rare: '#a855f7',
      Epic: '#ef4444',
      Legendary: '#f59e0b',
    };

    const color = rarityColors[dare.difficulty] || '#6366f1';

    const card = document.createElement('div');
    card.id = 'skribbl-dare-prompt-card';
    card.className = 'skribbl-dare-card';
    card.innerHTML = `
      <div class="skribbl-dare-header">
        <span class="skribbl-dare-badge" style="background: ${color}22; border-color: ${color}; color: ${color};">
          🎲 ${escapeHTML(dare.difficulty).toUpperCase()} DARE · +${dare.points} PTS
        </span>
        <button id="skribbl-dare-reroll" class="skribbl-dare-reroll-btn" title="Reroll dare">🔄 Reroll</button>
      </div>
      <div class="skribbl-dare-title">${escapeHTML(dare.text)}</div>
      <div class="skribbl-dare-hint">
        💡 Pick your word below and complete this dare to earn +${dare.points} pts!
      </div>
    `;

    // Insert directly before the word choices
    wordsContainer.parentNode.insertBefore(card, wordsContainer);

    // Attach click listeners to word options
    const wordBtns = wordsContainer.querySelectorAll('.word');
    wordBtns.forEach((btn) => {
      btn.addEventListener('click', function () {
        chosenWord = this.textContent.trim();
        console.log('🎯 Word chosen:', chosenWord, 'with dare:', dare.text);
        showFloatingHUD(dare);
        enableCompletedDareButton(dare);
      }, { once: true });
    });

    // Reroll button listener
    const rerollBtn = card.querySelector('#skribbl-dare-reroll');
    rerollBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      rerollBtn.disabled = true;
      rerollBtn.textContent = '⏳ ...';
      try {
        const newDare = await fetchDare();
        activeDare = newDare;
        renderDareSelectionCard(wordsContainer, newDare);
      } catch (err) {
        rerollBtn.textContent = 'Failed';
      }
    });
  }

  // --- 7. Floating Active Dare HUD (During Drawing) ---
  function showFloatingHUD(dare) {
    let hud = document.querySelector('#skribbl-dare-hud');
    if (!hud) {
      hud = document.createElement('div');
      hud.id = 'skribbl-dare-hud';
      hud.className = 'skribbl-dare-hud';
      document.body.appendChild(hud);
    }

    const rarityColors = {
      Common: '#10b981',
      Uncommon: '#3b82f6',
      Rare: '#a855f7',
      Epic: '#ef4444',
      Legendary: '#f59e0b',
    };
    const color = rarityColors[dare.difficulty] || '#6366f1';

    hud.innerHTML = `
      <div class="skribbl-dare-hud-top">
        <span class="skribbl-dare-hud-label" style="color: ${color};">
          ⚡ ACTIVE DARE · +${dare.points} PTS
        </span>
        <button class="skribbl-dare-hud-close" title="Hide HUD">✕</button>
      </div>
      <div class="skribbl-dare-hud-text">${escapeHTML(dare.text)}</div>
      <button id="btn-claim-hud" class="skribbl-dare-hud-claim-btn">
        <span>🎯 Completed Dare! Claim +${dare.points}</span>
      </button>
    `;
    hud.style.display = 'flex';

    hud.querySelector('.skribbl-dare-hud-close').addEventListener('click', () => {
      hud.style.display = 'none';
    });

    hud.querySelector('#btn-claim-hud').addEventListener('click', handleCompleteDareClick);
  }

  // --- 8. "Completed Dare" Action Button ---
  function injectCompletedDareButton() {
    if (document.querySelector('#btn-completed-dare')) return;

    const rateContainer = document.querySelector('#game-rate') ||
      document.querySelector('.rate-container') ||
      document.querySelector('#game-toolbar');

    if (!rateContainer) return;

    const btn = document.createElement('button');
    btn.id = 'btn-completed-dare';
    btn.className = 'skribbl-dare-complete-btn disabled';
    btn.innerHTML = `
      <span class="btn-icon">🎯</span>
      <span class="btn-text">Completed Dare</span>
    `;

    btn.addEventListener('click', handleCompleteDareClick);

    const likeBtn = rateContainer.querySelector('.like, .rate-like') || rateContainer.firstChild;
    if (likeBtn && likeBtn.parentNode === rateContainer) {
      rateContainer.insertBefore(btn, likeBtn.nextSibling);
    } else {
      rateContainer.appendChild(btn);
    }
  }

  function enableCompletedDareButton(dare) {
    const btn = document.querySelector('#btn-completed-dare');
    if (btn) {
      dareClaimedThisRound = false;
      btn.classList.remove('disabled', 'claimed');
      btn.innerHTML = `
        <span class="btn-icon">🎯</span>
        <span class="btn-text">Completed Dare (+${dare.points})</span>
      `;
    }

    const hudBtn = document.querySelector('#btn-claim-hud');
    if (hudBtn) {
      hudBtn.disabled = false;
      hudBtn.classList.remove('claimed');
      hudBtn.innerHTML = `<span>🎯 Completed Dare! Claim +${dare.points}</span>`;
    }
  }

  async function handleCompleteDareClick() {
    if (dareClaimedThisRound) {
      showToast('✅ Dare already claimed for this round!');
      return;
    }

    if (!activeDare) {
      showToast('⚠️ No active dare registered for this round!');
      return;
    }

    if (!currentToken) {
      showToast('🔒 Please log in with Discord or Dev mode on the home screen to earn points!');
      return;
    }

    const btn = document.querySelector('#btn-completed-dare');
    const hudBtn = document.querySelector('#btn-claim-hud');

    if (btn) {
      btn.classList.add('disabled');
      btn.innerHTML = `<span class="btn-icon">⏳</span><span class="btn-text">Claiming...</span>`;
    }
    if (hudBtn) {
      hudBtn.disabled = true;
      hudBtn.innerHTML = `<span>⏳ Claiming...</span>`;
    }

    try {
      const payload = {
        dare_id: activeDare.id || activeDare.ID,
        word: chosenWord || 'Unknown',
        room_code: getRoomCode()
      };

      chrome.runtime.sendMessage({ type: 'COMPLETE_DARE', payload }, (res) => {
        if (res && res.success) {
          const result = res.result;
          dareClaimedThisRound = true;

          if (btn) {
            btn.classList.remove('disabled');
            btn.classList.add('claimed');
            btn.innerHTML = `<span class="btn-icon">✅</span><span class="btn-text">Dare Completed! (+${result.points_earned} pts)</span>`;
          }
          if (hudBtn) {
            hudBtn.classList.add('claimed');
            hudBtn.innerHTML = `<span>✅ Claimed! (+${result.points_earned} pts)</span>`;
          }

          playCelebrationSound();
          triggerConfetti();

          showToast(`🎉 Dare Completed! +${result.points_earned} pts earned!`);
          if (result.tier_upgraded) {
            showToast(`🚀 PROMOTED! You unlocked: ${result.tier.name} (${result.tier.rarity})!`, 6000);
          }

          if (currentUser) {
            currentUser.points = result.total_points;
            currentUser.tier = result.tier;
          }
          updateAllUI();
        } else {
          if (btn) {
            btn.classList.remove('disabled');
            btn.innerHTML = `<span class="btn-icon">❌</span><span class="btn-text">Failed to claim</span>`;
          }
          if (hudBtn) {
            hudBtn.disabled = false;
            hudBtn.innerHTML = `<span>❌ Failed to claim</span>`;
          }
          showToast(`❌ ${res ? res.error : 'Failed to complete dare'}`);
          setTimeout(() => enableCompletedDareButton(activeDare), 2500);
        }
      });
    } catch (e) {
      if (btn) btn.classList.remove('disabled');
      if (hudBtn) hudBtn.disabled = false;
      showToast('❌ Error connecting to dare server');
    }
  }

  // --- 9. Player List Sticker & Tag Injection ---
  function updatePlayerBadges() {
    const playerRows = document.querySelectorAll('#game-players .player');
    if (!playerRows.length) return;

    playerRows.forEach((row) => {
      const nameEl = row.querySelector('.player-name, .name, .info .name');
      if (!nameEl) return;

      const rawNick = nameEl.childNodes[0] ? nameEl.childNodes[0].textContent.trim().toLowerCase() : '';
      if (!rawNick) return;

      let playerInfo = roomPlayers.get(rawNick);
      if (!playerInfo && currentUser && (rawNick === currentUser.username.toLowerCase() || row.classList.contains('me'))) {
        playerInfo = {
          username: currentUser.username,
          ingame_nickname: rawNick,
          points: currentUser.points,
          tier: currentUser.tier,
          avatar_url: currentUser.avatar_url
        };
      }

      if (!playerInfo || !playerInfo.tier) return;

      const stickerID = playerInfo.tier.sticker_id;
      const stickerSrc = STICKER_URLS[stickerID] || STICKER_URLS.warmup_artist;

      let badgeContainer = nameEl.querySelector('.skribbl-dare-player-badge');
      if (!badgeContainer) {
        badgeContainer = document.createElement('span');
        badgeContainer.className = 'skribbl-dare-player-badge';
        nameEl.appendChild(badgeContainer);
      }

      badgeContainer.innerHTML = `
        <img class="skribbl-dare-sticker" src="${stickerSrc}" alt="${playerInfo.tier.name}" title="${playerInfo.tier.name} (${playerInfo.tier.rarity}) · ${playerInfo.points} pts">
        <span class="skribbl-dare-tier-tag" style="background: ${playerInfo.tier.color_hex}22; border-color: ${playerInfo.tier.color_hex}; color: ${playerInfo.tier.color_hex};">
          ${playerInfo.tier.name}
        </span>
      `;
    });
  }

  // --- 10. UI Audio & Confetti Effects ---
  function playCelebrationSound() {
    try {
      const ctx = new (window.AudioContext || window.webkitAudioContext)();
      const notes = [523.25, 659.25, 783.99, 1046.50];
      notes.forEach((freq, idx) => {
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.type = 'triangle';
        osc.frequency.value = freq;
        gain.gain.setValueAtTime(0.15, ctx.currentTime + idx * 0.1);
        gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + idx * 0.1 + 0.35);
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.start(ctx.currentTime + idx * 0.1);
        osc.stop(ctx.currentTime + idx * 0.1 + 0.35);
      });
    } catch (e) {
      // AudioContext muted/unsupported
    }
  }

  function triggerConfetti() {
    const canvas = document.querySelector('#game-canvas canvas') || document.body;
    const rect = canvas.getBoundingClientRect();
    const count = 35;
    const colors = ['#f59e0b', '#ec4899', '#3b82f6', '#10b981', '#a855f7'];

    for (let i = 0; i < count; i++) {
      const confetti = document.createElement('div');
      confetti.className = 'skribbl-dare-confetti-particle';
      confetti.style.left = `${rect.left + rect.width / 2 + (Math.random() - 0.5) * 200}px`;
      confetti.style.top = `${rect.top + rect.height / 2 + (Math.random() - 0.5) * 100}px`;
      confetti.style.backgroundColor = colors[Math.floor(Math.random() * colors.length)];
      confetti.style.setProperty('--dx', `${(Math.random() - 0.5) * 350}px`);
      confetti.style.setProperty('--dy', `${(Math.random() - 1.2) * 280}px`);
      document.body.appendChild(confetti);

      setTimeout(() => confetti.remove(), 1200);
    }
  }

  function showToast(msg, duration = 3500) {
    const toast = document.createElement('div');
    toast.className = 'skribbl-dare-toast';
    toast.textContent = msg;
    document.body.appendChild(toast);
    setTimeout(() => toast.classList.add('show'), 10);
    setTimeout(() => {
      toast.classList.remove('show');
      setTimeout(() => toast.remove(), 400);
    }, duration);
  }

  function escapeHTML(str) {
    if (!str) return '';
    return String(str).replace(/[&<>'"]/g, tag => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      "'": '&#39;',
      '"': '&quot;'
    }[tag] || tag));
  }
})();
