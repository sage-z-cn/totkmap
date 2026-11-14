// English language pack (mirror of zh-CN.js — same keys, same nesting)
// Progress category names (progress.*) use the official in-game English terms
// Note: the 913 region names in regions.js are a data layer and are NOT translated
//      (region names stay Chinese even in the English UI)
export default {
  title: {
    main: 'TOTKNavi Locator & Navigator',
    nav: 'TOTKNavi'
  },
  header: {
    settings: 'Settings',
    about: 'About',
    minimize: 'Minimize',
    maximize: 'Maximize',
    restore: 'Restore',
    close: 'Close'
  },
  tabs: {
    control: 'Console',
    map: 'Map'
  },
  toolbar: {
    refresh: 'Refresh',
    openBrowser: 'Open in browser',
    mapOffline: 'Offline map',
    mapOnline: 'Online map',
    launchGame: 'Launch game',
    gameRunning: 'Game running',
    launching: 'Game launched (PID {pid})'
  },
  map: {
    starting: 'Starting location service…',
    startingHint: 'The core process is up; waiting for the local map port. This usually takes 1–2 seconds.',
    retry: 'Retry'
  },
  sync: {
    waiting: 'Waiting for coordinate stream',
    syncing: 'Syncing coordinate stream',
    attached: 'Attached pid={pid}, locating...',
    waitingProcess: 'Waiting for emulator process...',
    ready: 'Ready'
  },
  // Phone mirror readiness (status bar appReadyText)
  mirror: {
    noUrl: 'No mirror address detected',
    ready: 'Mirror ready',
    noIndex: 'Directory exists but index.html is missing',
    notFound: 'app/ directory not found',
    unknown: 'Status unknown'
  },
  steps: {
    ready: 'Ready',
    locating: 'Locating',
    verifying: 'Verifying movement',
    locked: 'Locked'
  },
  state: {
    ready: 'Ready',
    waitingProcess: 'Waiting for emulator process...',
    locating: 'Locating...',
    knownUnconfirmed: '⚡ Quick lock · awaiting movement',
    lockedUnconfirmed: 'Locked · awaiting movement',
    verified: 'Verified · following'
  },
  layer: {
    '18': 'Surface',
    '19': 'Depths',
    '20': 'Sky'
  },
  source: {
    known: '⚡ Quick lock',
    chain: '⚡ L1 Static chain',
    scan: '⚠ L2 Heap scan',
    coords: '📍 L3 Calibrated',
    probe: 'Probe regroup',
    consensus: 'Consensus recovery',
    locating: 'Locating'
  },
  emu: {
    auto: 'PC-Recomp',
    ryujinx: 'Ryujinx',
    eden: 'Eden',
    pickTitle: 'Select platform (PC edition uses pc-recomp)'
  },
  buttons: {
    openMap: 'Open map',
    start: 'Start locating',
    locating: 'Locating...',
    browse: 'Browse',
    openDir: 'Open folder',
    save: 'Save',
    checking: 'Checking...',
    cancel: 'Cancel',
    confirm: 'Continue anyway',
    later: 'Later',
    download: 'Download',
    clear: 'Clear',
    checkUpdate: 'Check for updates',
    exportDiag: 'Export diagnostics',
    chooseFile: 'Choose File',
    chooseDir: 'Choose Dir',
    locateFile: 'Locate File',
    detectSave: 'Auto Detect'
  },
  locate: {
    liveCoords: 'Live coordinates',
    offset: 'Offset',
    axisZ: 'Z height',
    currentLayer: 'Current layer',
    calibration: 'Coordinate calibration',
    calWhenFail: 'Use when locating fails',
    submit: 'Submit calibration',
    calHint: 'Enter in-game character coordinates; memory scan takes about 5-20s',
    placeholder: 'e.g. {v}',
    localProgress: 'Local save parsing',
    categoryCount: '{n} / 21 categories',
    waitingProgress: 'Waiting for save data... (start locating first)',
    noneProgress: 'No save data yet',
    preset: 'Presets',
    tower: 'Landing Tower',
    landing: 'Lookout Landing',
    northShrine: 'North Shrine',
    awakeningRoom: 'Awakening Room',
    saveInfo: 'Active save',
    latestSaveTime: 'Latest save',
    resetAutoDetectTip: '✓ Save detection: manual path cleared, restored auto-detection',
    justNow: 'Just now',
    minsAgo: '{m}m ago',
    hoursAgo: '{h}h ago',
    noSave: 'No save detected (auto-detected after start)'
  },
  precheck: {
    title: '⚠ Large coordinate difference',
    descPre: 'Your input differs from the current locked position by',
    currentLock: 'Locked',
    yourInput: 'Your input',
    tip1: '· Make sure you read the live coordinates at the bottom-right of the game (map closed), in X / Y / Z order',
    tip2: '· Red dot clearly misplaced (lock may be inaccurate) → click "Continue anyway"',
    tip3: '· Just a typo → click "Cancel" to go back'
  },
  cal: {
    needInput: '✗ Calibration: please enter in-game coordinates (X Y Z)',
    submitting: '↻ Calibration: submitting... (memory scan takes about 5-20s)',
    failed: '✗ Calibration failed: {err}',
    missDefault: 'No match. Stay still and retry',
    locked: '✓ Calibration locked {addr}{cp}',
    copies: '({n} copies matched)',
    timeout: 'Calibration timed out: no response from core (start locating first, then retry)',
    submitFailed: 'Submission failed'
  },
  emuSwitch: '⚠ Emulator switched: stop and restart locating for the change to take effect',
  log: {
    title: 'Live log',
    none: 'No logs yet',
    filter: 'Filter...',
    shown: '{s} / {t}'
  },
  settings: {
    title: 'Settings',
    ryuPath: 'Ryujinx save path:',
    edenPath: 'Eden save path:',
    workDir: 'Working directory (totknavi-core.exe logs / status.json):',
    exeDir: 'Same folder as exe (next to totknavi-core.exe)',
    pathAuto: 'Leave blank to auto-detect',
    tls: 'Keep phone awake (Https)',
    tlsWarn: '[Do NOT enable without installing the CA certificate on your phone!]',
    autoOpenMap: 'Switch to the map tab after locating succeeds',
    autoSwitchToMap: 'Switch to the map tab after locating succeeds',
    autoMoveToSecondary: 'Move TOTKnavi to the non-game screen and maximize after locating',
    autoMoveTarget: 'Target screen:',
    autoMoveTargetAuto: 'Auto (any screen except the game screen)',
    autoMoveTargetHint: 'The screen containing the game window is treated as primary; choose where TOTKnavi should go.',
    primaryMonitor: 'Primary',
    autoStartOnGame: 'Start locating automatically when the game launches',
    gamePath: 'Game path (totk.exe):',
    gamePathOk: '✓ Valid path, ready to launch',
    gamePathHint: 'Leave blank to auto-detect; click Browse if not found',
    saved: 'Saved',
    pathOk: '✓ Valid path',
    pathBad: 'Path does not exist (core falls back to auto-detect)',
    language: 'Language',
    langSystem: 'Follow system',
    langZh: '中文',
    langEn: 'English'
  },
  about: {
    title: 'About',
    subtitle: 'TOTKNavi · Zelda: Tears of the Kingdom (TOTK) locator & navigator',
    version: 'Version',
    map: 'Map',
    github: 'GitHub',
    readOnly: 'Read-only memory access. No injection, no game modification.',
    latest: 'Up to date',
    latestVer: '✓ Already the latest v{v}',
    checkFailed: '✗ {msg}'
  },
  update: {
    available: 'A new version is available',
    current: 'Current',
    latest: 'Latest'
  },
  progress: {
    skyview: 'Skyview Towers',
    tears: 'Dragon Tears',
    shrines: 'Shrines',
    lightroots: 'Lightroots',
    koroks: 'Koroks',
    koroksCarry: 'Carry Koroks',
    bubbuls: 'Bubbul Guts',
    treasureMaps: 'Old Maps',
    sageWills: "Sage's Wills",
    schemas: 'Schema Stones',
    addison: "Addison's Signs",
    chests: 'Chests',
    hinoxes: 'Hinoxes',
    taluses: 'Stone Taluses',
    moldugas: 'Moldugas',
    flux: 'Flux Constructs',
    frox: 'Frox',
    gleethok: 'Gleethoks',
    chasms: 'Chasms',
    caves: 'Caves',
    wells: 'Wells'
  },
  // Mapping base for fixed Go-side messages (see gomsg.js; keys here for cross-reference)
  go: {
    pickDirTitle: 'Choose save directory (optional, auto-detect by default)'
  }
}
