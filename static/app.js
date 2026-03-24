// ─── Questions Data ───────────────────────────────────────────────────────────
// Loaded from /api/questions (served by main.go)
// The QUESTIONS array below is kept here as fallback; server is the source of truth.

// ─── State ───────────────────────────────────────────────────────────────────
const API = 'http://localhost:8080/api';
let questions = [];
let currentQ = null;
let player = null;
let editor = null;
let timerInterval = null;
let startTime = null;
let filterDiff = 'all';
let lintTimeout = null;
let outputTab = 'output';
let lastOutput = '';
let lastError = '';
let dailyQ = null;

const PLAYER_KEY = 'gochallenge_player_id';

// ─── Init ────────────────────────────────────────────────────────────────────
function getOrCreateID() {
  let id = localStorage.getItem(PLAYER_KEY);
  if (!id) { id = 'p_' + Math.random().toString(36).slice(2); localStorage.setItem(PLAYER_KEY, id); }
  return id;
}

async function init() {
  const id = getOrCreateID();
  player = await fetch(`${API}/player?id=${id}`).then(r=>r.json()).catch(()=>({id,name:'Gopher',points:0,level:1,streak:0,achievements:[],solvedIds:[]}));
  questions = await fetch(`${API}/questions`).then(r=>r.json()).catch(()=>[]);
  dailyQ = await fetch(`${API}/daily`).then(r=>r.json()).catch(()=>null);
  updateHeader();
  renderList();
  initMonaco();
}

// ─── Monaco ──────────────────────────────────────────────────────────────────
function initMonaco() {
  require.config({ paths: { vs: 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.44.0/min/vs' } });
  require(['vs/editor/editor.main'], function() {
    monaco.editor.defineTheme('gochallenge', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'keyword', foreground: '00d4aa', fontStyle: 'bold' },
        { token: 'string', foreground: 'a8e6cf' },
        { token: 'comment', foreground: '4a5568', fontStyle: 'italic' },
        { token: 'number', foreground: 'fbbf24' },
        { token: 'type', foreground: '5b8ef0' },
        { token: 'function', foreground: 'a78bfa' },
      ],
      colors: {
        'editor.background': '#0d0f14',
        'editor.foreground': '#e8eaf0',
        'editor.lineHighlightBackground': '#1a1e28',
        'editor.selectionBackground': '#2a3a5a',
        'editorLineNumber.foreground': '#3a4260',
        'editorLineNumber.activeForeground': '#6b7594',
        'editor.indentGuide.background': '#1a1e28',
        'scrollbar.shadow': '#000000',
        'editorCursor.foreground': '#00d4aa',
      }
    });
    editor = monaco.editor.create(document.getElementById('editor'), {
      value: '// Select a challenge to begin\npackage main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("Hello, Gopher!")\n}',
      language: 'go',
      theme: 'gochallenge',
      fontSize: 14,
      fontFamily: "'JetBrains Mono', monospace",
      fontLigatures: true,
      lineNumbers: 'on',
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      automaticLayout: true,
      tabSize: 2,
      wordWrap: 'off',
      smoothScrolling: true,
      cursorBlinking: 'smooth',
      cursorSmoothCaretAnimation: true,
      renderLineHighlight: 'line',
      padding: { top: 12, bottom: 12 },
      suggest: { showKeywords: true },
    });

    editor.onDidChangeModelContent(() => {
      clearTimeout(lintTimeout);
      setLint('checking');
      lintTimeout = setTimeout(() => lintCode(), 1200);
    });
  });
}

// ─── Lint ─────────────────────────────────────────────────────────────────────
let lintErrors = [];

async function lintCode() {
  if (!editor) return;
  const code = editor.getValue();
  try {
    const res = await fetch(`${API}/lint`, {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({code})
    });
    const data = await res.json();
    lintErrors = data.errors || [];
    if (lintErrors.length > 0) {
      setLint('error', lintErrors.length + ' issue(s)');
      showLintErrors(lintErrors);
    } else {
      setLint('ok');
    }
  } catch { setLint('ok'); }
}

function showLintErrors(errors) {
  const out = document.getElementById('output-content');
  out.innerHTML = '<span class="out-error">⚠ Lint errors:</span>\n' +
    errors.map(e => `<span style="color:var(--red)">${escHtml(e)}</span>`).join('\n');
}

function setLint(state, msg='') {
  const dot = document.getElementById('lint-dot');
  const txt = document.getElementById('lint-text');
  dot.className = 'lint-dot';
  if (state === 'error') {
    dot.classList.add('error');
    txt.textContent = msg || 'Errors';
    txt.style.color = 'var(--red)';
    txt.style.cursor = 'pointer';
    txt.onclick = () => lintErrors.length && showLintErrors(lintErrors);
  } else if (state === 'checking') {
    dot.classList.add('checking');
    txt.textContent = 'Checking…';
    txt.style.color = 'var(--yellow)';
    txt.style.cursor = 'default';
    txt.onclick = null;
  } else {
    txt.textContent = 'Clean';
    txt.style.color = 'var(--green)';
    txt.style.cursor = 'default';
    txt.onclick = null;
  }
}

// ─── Questions ───────────────────────────────────────────────────────────────
function renderList() {
  const search = document.getElementById('q-search').value.toLowerCase();
  const list = document.getElementById('q-list');
  const solvedIds = player?.solvedIds || [];

  const filtered = questions.filter(q => {
    const matchDiff = filterDiff === 'all' || q.difficulty === filterDiff;
    const matchSearch = !search || q.title.toLowerCase().includes(search) || q.category.toLowerCase().includes(search);
    return matchDiff && matchSearch;
  });

  list.innerHTML = filtered.map(q => {
    const solved = solvedIds.includes(q.id);
    const isDaily = dailyQ && dailyQ.id === q.id;
    const active = currentQ && currentQ.id === q.id;
    return `<div class="q-item ${active?'active':''} ${solved?'solved':''}" onclick="selectQ(${q.id})">
      <span class="q-num">${q.id}</span>
      <div class="q-info">
        <div class="q-title">${isDaily ? '⭐ ' : ''}${q.title}</div>
        <div class="q-meta">${q.category} · ${q.points}pts</div>
      </div>
      <span class="diff-badge diff-${q.difficulty}">${q.difficulty[0].toUpperCase()}</span>
      ${solved ? '<span class="solved-check">✓</span>' : ''}
    </div>`;
  }).join('');

  document.getElementById('solved-count').textContent = `${solvedIds.length}/100`;
}

function setFilter(diff, btn) {
  filterDiff = diff;
  document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  renderList();
}

function selectQ(id) {
  currentQ = questions.find(q => q.id === id);
  if (!currentQ) return;

  document.getElementById('q-title').textContent = `#${currentQ.id} ${currentQ.title}`;
  document.getElementById('q-desc').textContent = currentQ.description;
  document.getElementById('q-pts').textContent = currentQ.points;

  const isDaily = dailyQ && dailyQ.id === currentQ.id;
  document.getElementById('daily-banner').style.display = isDaily ? 'flex' : 'none';

  // Hints
  const hintsRow = document.getElementById('hints-row');
  if (currentQ.hints && currentQ.hints.length > 0) {
    hintsRow.style.display = 'flex';
    const container = document.getElementById('hints-container');
    container.innerHTML = currentQ.hints.map((h, i) =>
      `<span class="hint-chip" id="hint-${i}" onclick="revealHint(${i},'${escapeHtml(h)}')">Hint ${i+1}</span>`
    ).join('');
  } else {
    hintsRow.style.display = 'none';
  }

  if (editor) editor.setValue(currentQ.template || '');
  document.getElementById('run-btn').disabled = false;
  document.getElementById('output-content').innerHTML = '<span class="out-info">// Ready to run. Click ▶ Run Code or press Ctrl+Enter</span>';
  renderList();
  startTimer();
}

function revealHint(i, text) {
  const chip = document.getElementById(`hint-${i}`);
  if (chip && !chip.classList.contains('revealed')) {
    chip.classList.add('revealed');
    chip.textContent = text;
    chip.title = text;
  }
}

function escapeHtml(s) { return s.replace(/'/g,"&#39;").replace(/"/g,'&quot;'); }

function resetCode() {
  if (!currentQ || !editor) return;
  editor.setValue(currentQ.template || '');
  startTimer();
}

// ─── Timer ───────────────────────────────────────────────────────────────────
function startTimer() {
  clearInterval(timerInterval);
  startTime = Date.now();
  updateTimerDisplay();
  timerInterval = setInterval(updateTimerDisplay, 1000);
}

function updateTimerDisplay() {
  if (!startTime) return;
  const elapsed = Math.floor((Date.now() - startTime) / 1000);
  const m = Math.floor(elapsed / 60);
  const s = elapsed % 60;
  const t = document.getElementById('timer');
  t.textContent = `${String(m).padStart(2,'0')}:${String(s).padStart(2,'0')}`;
  t.className = 'timer-display';
  if (elapsed > 300) t.classList.add('danger');
  else if (elapsed > 120) t.classList.add('warning');
}

// ─── Run Code ─────────────────────────────────────────────────────────────────
async function runCode() {
  if (!editor || !currentQ) return;
  const code = editor.getValue();
  const btn = document.getElementById('run-btn');
  btn.disabled = true;
  btn.textContent = '⏳ Running…';

  const out = document.getElementById('output-content');
  out.innerHTML = '<span class="out-info">⏳ Compiling and running…</span>';

  try {
    const res = await fetch(`${API}/run`, {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({
        code,
        questionId: currentQ.id,
        playerId: player.id,
        startTime: startTime || Date.now()
      })
    });
    const data = await res.json();
    lastOutput = data.output || '';
    lastError = data.error || '';

    if (data.success) {
      out.innerHTML = `<span class="out-success">✅ Success!</span>\n${escHtml(data.output)}`;
      if (data.points > 0) {
        out.innerHTML += `\n<span class="out-pts">+${data.points} points earned!</span>`;
        if (data.timeBonus > 0) {
          out.innerHTML += ` <span class="out-pts">(⚡ +${data.timeBonus} speed bonus)</span>`;
        }
      }
      player = await fetch(`${API}/player?id=${player.id}`).then(r=>r.json());
      updateHeader();
      renderList();
      clearInterval(timerInterval);

      if (data.unlocked && data.unlocked.length > 0) {
        data.unlocked.forEach(a => toast('achievement', `${a.icon} Achievement Unlocked!`, a.name + ' — ' + a.description));
      }
      if (data.points > 0) {
        toast('success', '✅ Challenge Solved!', `+${data.points} points${data.timeBonus ? ` (⚡ speed bonus)` : ''}`);
      }
    } else {
      out.innerHTML = `<span class="out-error">❌ Error:</span>\n<span style="color:var(--red)">${escHtml(data.error)}</span>`;
      toast('error', '❌ Build Failed', 'Check the output panel for details');
    }
  } catch(e) {
    out.innerHTML = `<span class="out-error">Connection error — is the server running?</span>`;
  }

  btn.disabled = false;
  btn.innerHTML = '<span>▶</span> Run Code';
}

function escHtml(s) {
  return (s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

function showTab(tab, el) {
  outputTab = tab;
  document.querySelectorAll('.output-tab').forEach(t=>t.classList.remove('active'));
  el.classList.add('active');
  const out = document.getElementById('output-content');
  if (tab === 'output') {
    out.innerHTML = lastOutput ? `<span class="out-success">Output:</span>\n${escHtml(lastOutput)}` : '<span class="out-info">// No output yet</span>';
  } else {
    out.innerHTML = lastError ? `<span class="out-error">Error:</span>\n<span style="color:var(--red)">${escHtml(lastError)}</span>` : '<span class="out-success">// No errors</span>';
  }
}

// ─── Ctrl+Enter shortcut ──────────────────────────────────────────────────────
document.addEventListener('keydown', e => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); runCode(); }
});

// ─── Leaderboard ─────────────────────────────────────────────────────────────
async function openLeaderboard() {
  document.getElementById('lb-panel').classList.add('open');
  const body = document.getElementById('lb-body');
  body.innerHTML = 'Loading…';
  const entries = await fetch(`${API}/leaderboard`).then(r=>r.json()).catch(()=>[]);
  if (!entries.length) { body.innerHTML = '<div style="color:var(--text3);text-align:center;padding:30px">No players yet. Be the first!</div>'; return; }
  const ranks = ['🥇','🥈','🥉'];
  body.innerHTML = entries.map(e => `
    <div class="lb-entry ${e.rank<=3?'top'+e.rank:''}">
      <div class="lb-rank">${ranks[e.rank-1]||e.rank}</div>
      <div class="lb-name">${escHtml(e.name)}</div>
      <div class="lb-lvl">Lv.${e.level}</div>
      <div class="lb-streak">🔥${e.streak}</div>
      <div class="lb-pts">${e.points} pts</div>
    </div>
  `).join('');
}

// ─── Achievements ─────────────────────────────────────────────────────────────
const ALL_ACHS = [
  // ── Milestones ───────────────────────────────
  {id:'first_blood',      name:'First Blood',        desc:'Solve your first challenge',          icon:'🩸'},
  {id:'gopher',           name:'Gopher',             desc:'Solve 10 challenges',                 icon:'🐹'},
  {id:'master_gopher',    name:'Master Gopher',      desc:'Solve 25 challenges',                 icon:'🦫'},
  {id:'legend',           name:'Legend',             desc:'Solve 50 challenges',                 icon:'👑'},
  {id:'centurion',        name:'Centurion',          desc:'Solve all 100 challenges',            icon:'🎯'},
  // ── Points ───────────────────────────────────
  {id:'pts_100',          name:'Rookie',             desc:'Earn 100 points',                     icon:'🌱'},
  {id:'pts_500',          name:'Rising Star',        desc:'Earn 500 points',                     icon:'⭐'},
  {id:'pts_1000',         name:'Centurion',          desc:'Earn 1000 points',                    icon:'🏆'},
  {id:'pts_2500',         name:'Elite',              desc:'Earn 2500 points',                    icon:'💫'},
  {id:'pts_5000',         name:'Grand Master',       desc:'Earn 5000 points',                    icon:'🌟'},
  // ── Speed ────────────────────────────────────
  {id:'speed_demon',      name:'Speed Demon',        desc:'Solve a challenge in under 60s',      icon:'⚡'},
  {id:'lightning',        name:'Lightning Fingers',  desc:'Solve a challenge in under 30s',      icon:'🌩️'},
  {id:'blink',            name:'Blink',              desc:'Solve a challenge in under 10s',      icon:'👁️'},
  // ── Streaks ──────────────────────────────────
  {id:'on_fire',          name:'On Fire',            desc:'5-challenge win streak',              icon:'🔥'},
  {id:'unstoppable',      name:'Unstoppable',        desc:'10-challenge win streak',             icon:'🚀'},
  {id:'godlike',          name:'Godlike',            desc:'25-challenge win streak',             icon:'🌈'},
  // ── Difficulty ───────────────────────────────
  {id:'easy_rider',       name:'Easy Rider',         desc:'Solve 10 easy challenges',            icon:'🛵'},
  {id:'middleway',        name:'Middle Way',         desc:'Solve 10 medium challenges',          icon:'⚖️'},
  {id:'perfectionist',    name:'Perfectionist',      desc:'Solve your first hard challenge',     icon:'💎'},
  {id:'hard_boiled',      name:'Hard Boiled',        desc:'Solve 10 hard challenges',            icon:'🥚'},
  {id:'no_easy_way',      name:'No Easy Way',        desc:'Solve 5 hard challenges in a row',   icon:'🪨'},
  // ── Categories ───────────────────────────────
  {id:'concurrency_king', name:'Concurrency King',   desc:'Solve a concurrency challenge',       icon:'🔀'},
  {id:'string_wizard',    name:'String Wizard',      desc:'Solve 5 string challenges',           icon:'🧵'},
  {id:'algo_master',      name:'Algo Master',        desc:'Solve 5 algorithm challenges',        icon:'🧠'},
  {id:'data_hoarder',     name:'Data Hoarder',       desc:'Solve 5 data-structure challenges',  icon:'🗄️'},
  {id:'generic_hero',     name:'Generic Hero',       desc:'Solve a generics challenge',          icon:'🦸'},
  {id:'web_dev',          name:'Web Dev',            desc:'Solve a web challenge',               icon:'🌐'},
  {id:'map_maker',        name:'Map Maker',          desc:'Solve 3 map challenges',              icon:'🗺️'},
  {id:'recursion_lord',   name:'Recursion Lord',     desc:'Solve 3 recursion challenges',        icon:'🌀'},
  // ── Time of day ──────────────────────────────
  {id:'night_owl',        name:'Night Owl',          desc:'Code past midnight',                  icon:'🦉'},
  {id:'early_bird',       name:'Early Bird',         desc:'Code before 6am',                     icon:'🐦'},
  {id:'lunch_coder',      name:'Lunch Coder',        desc:'Solve a challenge between 12–1pm',   icon:'🥪'},
  // ── Daily ────────────────────────────────────
  {id:'daily_warrior',    name:'Daily Warrior',      desc:'Complete a daily challenge',          icon:'📅'},
  {id:'weekly_grind',     name:'Weekly Grind',       desc:'Complete 7 daily challenges',         icon:'📆'},
  // ── Special ──────────────────────────────────
  {id:'comeback',         name:'Comeback Kid',       desc:'Solve a challenge after failing 3×',  icon:'💪'},
  {id:'hint_free',        name:'Hint Free',          desc:'Solve 10 challenges without hints',   icon:'🙈'},
  {id:'minimalist',       name:'Minimalist',         desc:'Solve a challenge with <10 lines',    icon:'✂️'},
  {id:'polyglot',         name:'Gopher Polyglot',    desc:'Use 5 different categories',          icon:'🦜'},
  {id:'collector',        name:'Collector',          desc:'Unlock 10 achievements',              icon:'🎁'},
];

function openAchievements() {
  document.getElementById('ach-panel').classList.add('open');
  const unlockedIds = new Set((player?.achievements||[]).map(a=>a.id));
  document.getElementById('ach-body').innerHTML = `
    <div style="margin-bottom:12px;color:var(--text2);font-size:13px">${unlockedIds.size}/${ALL_ACHS.length} unlocked</div>
    <div class="ach-grid">
      ${ALL_ACHS.map(a => `
        <div class="ach-card ${unlockedIds.has(a.id)?'unlocked':'locked'}">
          <div class="ach-icon">${a.icon}</div>
          <div class="ach-name">${a.name}</div>
          <div class="ach-desc">${a.desc}</div>
        </div>
      `).join('')}
    </div>
  `;
}

// ─── Profile ─────────────────────────────────────────────────────────────────
function openProfile() {
  document.getElementById('profile-panel').classList.add('open');
  document.getElementById('name-input').value = player?.name || '';
  const s = player;
  document.getElementById('profile-stats').innerHTML = `
    <div class="stat-card"><div class="stat-val">${s?.points||0}</div><div class="stat-lbl">Points</div></div>
    <div class="stat-card"><div class="stat-val">${s?.level||1}</div><div class="stat-lbl">Level</div></div>
    <div class="stat-card"><div class="stat-val">${s?.streak||0}</div><div class="stat-lbl">Streak</div></div>
    <div class="stat-card"><div class="stat-val">${(s?.solvedIds||[]).length}</div><div class="stat-lbl">Solved</div></div>
  `;
}

async function saveName() {
  const name = document.getElementById('name-input').value.trim();
  if (!name) return;
  player = await fetch(`${API}/player/update`, {
    method: 'POST',
    headers: {'Content-Type':'application/json'},
    body: JSON.stringify({id: player.id, name})
  }).then(r=>r.json());
  updateHeader();
  closePanel('profile-panel');
  toast('success', '✅ Name updated', `Now displaying as "${name}"`);
}

function openDaily() {
  if (dailyQ) selectQ(dailyQ.id);
}

// ─── Header Update ────────────────────────────────────────────────────────────
function updateHeader() {
  document.getElementById('hdr-name').textContent = player?.name || 'Gopher';
  document.getElementById('hdr-pts').textContent = (player?.points||0) + ' pts';
  document.getElementById('hdr-lvl').textContent = 'Lv.' + (player?.level||1);
}

// ─── Panels ───────────────────────────────────────────────────────────────────
function closePanel(id) { document.getElementById(id).classList.remove('open'); }
document.querySelectorAll('.panel-overlay').forEach(p => {
  p.addEventListener('click', e => { if (e.target === p) p.classList.remove('open'); });
});

// ─── Toast ────────────────────────────────────────────────────────────────────
function toast(type, title, msg) {
  const icons = { success: '✅', error: '❌', achievement: '🎖' };
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.innerHTML = `<div class="toast-icon">${icons[type]||'ℹ️'}</div><div class="toast-body"><div class="toast-title">${title}</div><div class="toast-msg">${msg}</div></div>`;
  document.getElementById('toasts').appendChild(el);
  setTimeout(() => {
    el.style.animation = 'slideOut 0.3s ease-in forwards';
    setTimeout(() => el.remove(), 300);
  }, 3500);
}

// ─── Start ────────────────────────────────────────────────────────────────────
init();