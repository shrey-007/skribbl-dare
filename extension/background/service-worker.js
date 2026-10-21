const API_BASE = 'https://deafening-overtime-penknife.ngrok-free.dev';

// Listen for messages from Content Scripts and Popup
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'GET_API_BASE') {
    sendResponse({ apiBase: API_BASE });
    return true;
  }

  if (message.type === 'DEV_LOGIN') {
    handleDevLogin(message.username, message.points)
      .then(data => sendResponse({ success: true, data }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true; // Keep channel open for async response
  }

  if (message.type === 'GET_AUTH_STATE') {
    chrome.storage.local.get(['skribbl_dare_jwt', 'skribbl_dare_user'], (res) => {
      sendResponse({
        token: res.skribbl_dare_jwt || null,
        user: res.skribbl_dare_user || null
      });
    });
    return true;
  }

  if (message.type === 'LOGOUT') {
    chrome.storage.local.remove(['skribbl_dare_jwt', 'skribbl_dare_user'], () => {
      sendResponse({ success: true });
    });
    return true;
  }

  if (message.type === 'FETCH_RANDOM_DARE') {
    fetchRandomDare()
      .then(dare => sendResponse({ success: true, dare }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'START_ROUND_DARE') {
    startRoundDare(message.payload)
      .then(res => sendResponse({ success: true, data: res }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'VOTE_DARE') {
    submitVote(message.payload)
      .then(res => sendResponse({ success: true, data: res }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'GET_ROUND_DARE') {
    getRoundDare(message.room_code)
      .then(res => sendResponse({ success: true, data: res }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true;
  }

  if (message.type === 'COMPLETE_DARE') {
    submitDareCompletion(message.payload)
      .then(result => sendResponse({ success: true, result }))
      .catch(err => sendResponse({ success: false, error: err.message }));
    return true;
  }
});

async function handleDevLogin(username, points) {
  const resp = await fetch(`${API_BASE}/api/auth/dev-login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, points })
  });

  if (!resp.ok) {
    throw new Error(`Login failed: ${resp.statusText}`);
  }

  const data = await resp.json();
  if (data.token) {
    await chrome.storage.local.set({
      skribbl_dare_jwt: data.token,
      skribbl_dare_user: data.user
    });
  }
  return data;
}

async function fetchRandomDare() {
  const { skribbl_dare_jwt } = await chrome.storage.local.get('skribbl_dare_jwt');
  const headers = { 'ngrok-skip-browser-warning': 'true' };
  if (skribbl_dare_jwt) {
    headers['Authorization'] = `Bearer ${skribbl_dare_jwt}`;
  }

  const resp = await fetch(`${API_BASE}/api/dares/random`, { headers });
  if (!resp.ok) {
    throw new Error('Failed to fetch dare from server');
  }

  const data = await resp.json();
  return data.dare;
}

async function submitDareCompletion(payload) {
  const { skribbl_dare_jwt } = await chrome.storage.local.get('skribbl_dare_jwt');
  if (!skribbl_dare_jwt) {
    throw new Error('You must be logged in to claim dare points! Open the extension popup to log in.');
  }

  const resp = await fetch(`${API_BASE}/api/dares/complete`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${skribbl_dare_jwt}`,
      'ngrok-skip-browser-warning': 'true'
    },
    body: JSON.stringify(payload)
  });

  if (!resp.ok) {
    const errorText = await resp.text();
    throw new Error(errorText || 'Failed to complete dare');
  }

  const data = await resp.json();

  // Refresh cached user profile
  try {
    const meResp = await fetch(`${API_BASE}/api/auth/me`, {
      headers: {
        'Authorization': `Bearer ${skribbl_dare_jwt}`,
        'ngrok-skip-browser-warning': 'true'
      }
    });
    if (meResp.ok) {
      const meData = await meResp.json();
      await chrome.storage.local.set({ skribbl_dare_user: meData.user });
    }
  } catch (e) {
    console.warn('Could not refresh user profile:', e);
  }

  return data;
}

async function startRoundDare(payload) {
  const resp = await fetch(`${API_BASE}/api/dares/start-round`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'ngrok-skip-browser-warning': 'true'
    },
    body: JSON.stringify(payload)
  });
  if (!resp.ok) {
    throw new Error('Failed to broadcast round dare');
  }
  return await resp.json();
}

async function submitVote(payload) {
  const resp = await fetch(`${API_BASE}/api/dares/vote`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'ngrok-skip-browser-warning': 'true'
    },
    body: JSON.stringify(payload)
  });
  if (!resp.ok) {
    throw new Error('Failed to submit dare vote');
  }
  return await resp.json();
}

async function getRoundDare(roomCode) {
  const resp = await fetch(`${API_BASE}/api/dares/current-round/${roomCode}`, {
    headers: { 'ngrok-skip-browser-warning': 'true' }
  });
  if (!resp.ok) {
    throw new Error('Failed to get round dare');
  }
  return await resp.json();
}
