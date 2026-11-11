// 中文语言包（默认源，与现有界面文案逐条一致——切换中文必须零视觉变化）
// 进度类别名（progress.*）= core progressCounts 的 key 英译基准，与 progress_totk2.go 对齐
// 注意：regions.js 的 913 个区域名是数据层，不做翻译（英文界面区域名显示中文）
export default {
  title: {
    main: 'TOTKNavi 定位导航',
    nav: 'TOTKNavi'
  },
  header: {
    settings: '设置',
    about: '关于',
    minimize: '最小化',
    maximize: '最大化',
    restore: '还原',
    close: '关闭'
  },
  tabs: {
    control: '控制台',
    map: '地图'
  },
  toolbar: {
    refresh: '刷新',
    openBrowser: '在浏览器打开',
    mapOffline: '离线地图',
    mapOnline: '在线地图',
    mapNone: '未连接',
    launchGame: '启动游戏',
    gameRunning: '游戏运行中',
    launching: '已启动游戏（PID {pid}）'
  },
  map: {
    notRunning: '定位服务未启动',
    notRunningHint: '启动后可在本窗口内直接查看实时位置，无需另开浏览器。',
    startService: '启动定位服务',
    starting: '定位服务启动中…',
    startingHint: '核心进程已启动，正在等待网页地图端口就绪；通常 1~2 秒即可。',
    retry: '重试',
    portBusy: '网页地图端口被占用',
    portBusyHint: '本机 8766 端口被其他程序占用，内嵌地图无法加载；关闭占用该端口的程序后点「刷新」重试，或先在浏览器里打开。'
  },
  sync: {
    waiting: '等待坐标流',
    syncing: '坐标流同步中',
    attached: '已附加 pid={pid}，定位中...',
    waitingProcess: '等待模拟器进程...',
    ready: '就绪'
  },
  // 手机镜像就绪状态（状态栏 appReadyText）
  mirror: {
    noUrl: '未检测到镜像地址',
    ready: '镜像已就绪',
    noIndex: '目录存在但缺 index.html',
    notFound: '未找到 app/ 目录',
    unknown: '状态未知'
  },
  steps: {
    ready: '就绪',
    locating: '定位中',
    verifying: '验证移动',
    locked: '已锁定'
  },
  state: {
    ready: '就绪',
    waitingProcess: '等待模拟器进程...',
    locating: '定位中...',
    knownUnconfirmed: '⚡ 秒锁 · 待移动确认',
    lockedUnconfirmed: '已锁定 · 待移动确认',
    verified: '已验证 · 跟随中'
  },
  layer: {
    '18': '地上',
    '19': '地下',
    '20': '天空'
  },
  source: {
    known: '⚡ 秒锁',
    chain: '⚡ L1 静态直读',
    scan: '⚠ L2 扫描降级',
    coords: '📍 L3 手动校准',
    probe: '探针换组',
    consensus: '共识恢复',
    locating: '定位中'
  },
  emu: {
    auto: 'PC-Recomp',
    ryujinx: 'Ryujinx',
    eden: 'Eden',
    pickTitle: '选择平台（PC版默认使用 pc-recomp）'
  },
  buttons: {
    openMap: '打开地图',
    start: '开始定位',
    locating: '正在定位中',
    browse: '浏览',
    openDir: '打开目录',
    save: '保存',
    checking: '检查中...',
    cancel: '取消',
    confirm: '确认继续',
    later: '稍后',
    download: '下载',
    clear: '清屏',
    checkUpdate: '检查更新',
    exportDiag: '导出诊断包',
    chooseFile: '选择存档',
    chooseDir: '选择目录',
    locateFile: '定位文件',
    detectSave: '重新检测'
  },
  locate: {
    liveCoords: '实时同步坐标',
    offset: '偏移量',
    axisZ: 'Z 高度',
    currentLayer: '所在图层',
    calibration: '坐标校准',
    calWhenFail: '定位失败时使用',
    submit: '提交校准',
    calHint: '填写游戏内角色坐标，校准扫描内存约 5-20s',
    placeholder: '如 {v}',
    localProgress: '本地解析进度',
    categoryCount: '{n} / 21 类',
    waitingProgress: '等待读取存档...（需先开始定位）',
    noneProgress: '暂无存档数据',
    preset: '预设',
    tower: '堡垒鸟望台',
    landing: '监视堡垒',
    northShrine: '堡垒北神庙',
    awakeningRoom: '觉醒室',
    saveInfo: '生效存档',
    latestSaveTime: '最新存档',
    resetAutoDetectTip: '✓ 存档检测：已清除手动路径，恢复全自动探测最新存档',
    justNow: '刚刚',
    minsAgo: '{m}分钟前',
    hoursAgo: '{h}小时前',
    noSave: '未检测到存档（开始定位后自动探测）'
  },
  precheck: {
    title: '⚠ 坐标差异较大',
    descPre: '你填的坐标与当前锁定位置相差',
    currentLock: '当前锁定',
    yourInput: '你填的',
    tip1: '· 确认读的是游戏右下角实时坐标（不打开地图），三位数字按 X / Y / Z 顺序',
    tip2: '· 红点位置明显不对（锁定可能不准）→ 点"确认继续"',
    tip3: '· 只是打错字 → 点"取消"回去改'
  },
  cal: {
    needInput: '✗ 坐标校准：请填写游戏内坐标（X Y Z）',
    submitting: '↻ 坐标校准：提交中…（扫描内存约 5-20s）',
    failed: '✗ 坐标校准失败：{err}',
    missDefault: '未命中，请保持站桩重试',
    locked: '✓ 坐标校准已锁定 {addr}{cp}',
    copies: '（匹配 {n} 个副本）',
    timeout: '校准超时：core 未响应（请确认已开始定位后重试）',
    submitFailed: '提交失败'
  },
  emuSwitch: '⚠ 模拟器已切换：请停止后重新开始定位生效',
  log: {
    title: '实时日志',
    none: '暂无日志',
    filter: '过滤...',
    shown: '{s} / {t}'
  },
  settings: {
    title: '设置',
    ryuPath: 'Ryujinx 存档路径：',
    edenPath: 'Eden 存档路径：',
    workDir: '工作目录（totknavi-core.exe 的日志 / status.json 所在）：',
    exeDir: 'exe 同目录（与 totknavi-core.exe 一起）',
    pathAuto: '留空自动探测',
    tls: '手机防息屏（Https）',
    tlsWarn: '【手机没装 CA 证书，不要勾选！】',
    autoOpenMap: '定位成功后自动切换到地图页',
    autoSwitchToMap: '定位成功后自动切换到地图页',
    autoMoveToSecondary: '定位后自动移到副屏并最大化（游戏所在屏视为主屏）',
    autoMoveTarget: '目标副屏：',
    autoMoveTargetAuto: '自动（游戏所在屏之外）',
    autoMoveTargetHint: '游戏窗口所在屏视为主屏；这里选择 TOTKnavi 要去的屏幕。',
    primaryMonitor: '主屏',
    autoStartOnGame: '游戏启动后自动开始定位',
    gamePath: '游戏路径（totk.exe）：',
    gamePathOk: '✓ 路径有效，可直接启动',
    gamePathHint: '留空自动探测；找不到时点「浏览」选择 totk.exe',
    saved: '已保存',
    pathOk: '✓ 路径有效',
    pathBad: '路径不存在（core 将回退自动探测）',
    language: '语言',
    langSystem: '跟随系统',
    langZh: '中文',
    langEn: 'English'
  },
  about: {
    title: '关于',
    subtitle: 'TOTKNavi · 王国之泪 (TOTK) 定位导航程序',
    version: '版本',
    map: '地图',
    github: 'GitHub',
    readOnly: '只读内存，不注入，不修改游戏。',
    latest: '已是最新版本',
    latestVer: '✓ 已是最新版 v{v}',
    checkFailed: '✗ {msg}'
  },
  update: {
    available: '有新版本可用',
    current: '当前',
    latest: '最新'
  },
  progress: {
    skyview: '鸟望台',
    tears: '龙之泪',
    shrines: '神庙',
    lightroots: '树根',
    koroks: '克洛格',
    koroksCarry: '双倍克洛格',
    bubbuls: '魔犹伊遗失物',
    treasureMaps: '残旧的地图',
    sageWills: '贤者的遗志',
    schemas: '设计图石板',
    addison: '卡邦达立牌',
    chests: '宝箱',
    hinoxes: '独眼巨人',
    taluses: '岩石巨人',
    moldugas: '莫尔德拉吉克',
    flux: '方块魔像',
    frox: '巨霸伽马',
    gleethok: '古栗欧克',
    chasms: '地洞入口',
    caves: '洞穴入口',
    wells: '井'
  },
  // Go 端固定消息的映射基准（见 gomsg.js；这里放 key 便于两包对照）
  go: {
    pickDirTitle: '选择存档目录（可选，默认自动探测）'
  }
}
