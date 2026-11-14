/* ============================================================
 * 龙实时位置图层（dragon-live.js）
 * ------------------------------------------------------------
 * 数据：totknavi / totknavi-pc 的 GET /dragons
 *   { ok, items:[{id,name,gx,gy,gz,mx,my,layer,simulated,locked,updated}] }
 * 渲染：按当前地图层（18 地上 / 19 地下 / 20 天空）显示已锁定且同层的龙；
 *       simulated=false 时用空心圈表示"该龙不在附近、坐标为最后缓存"。
 * 开关：跟随当前层「龙的轨迹」分类（与 dragon-layer.js 同一开关），
 *       关闭时实时图标一并隐藏；不再提供右上角独立按钮与校准入口。
 * 说明：龙的飞行轨迹由已有的 dragon-layer.js 负责（侧栏「龙的轨迹」分类），
 *       本模块只画实时位置标记，不重复画线。
 * ============================================================ */
(function () {
  'use strict';

  var CAT_ID = { 18: 229, 19: 230, 20: 231 };
  var LAYER_NAME = { 18: '地上', 19: '地下', 20: '天空' };
  var DRAGONS = [
    { id: 'light', name: '白龙' },
    { id: 'dinraal', name: '奥尔龙' },
    { id: 'naydra', name: '聂尔龙' },
    { id: 'farosh', name: '费罗龙' }
  ];
  var COLORS = {
    light: '#e8e6f0', dinraal: '#ff6b4a', naydra: '#6ec8ff', farosh: '#ffe066'
  };

  var API = (function () {
    var qs = new URLSearchParams(location.search);
    if (qs.get('dragonApi')) return 'http://127.0.0.1:' + qs.get('dragonApi');
    // 页面由定位服务托管时同源；否则默认 8766。
    if (location.port === '8766' || location.port === '8777') return location.origin;
    return 'http://127.0.0.1:8766';
  })();

  var map = null;
  var group = null;
  var lastItems = [];
  var lastOn = null;
  var lastLayer = null;

  function currentLayer() {
    if (window.TOTK && window.TOTK.state && window.TOTK.state.layer != null) {
      return window.TOTK.state.layer;
    }
    var active = document.querySelector('#layerSwitch button.active');
    return active ? Number(active.getAttribute('data-layer')) : 18;
  }

  /* 与 dragon-layer.js 的 isOn() 同口径：当前层勾了「龙的轨迹」= 显示。 */
  function trailOn() {
    var s = window.TOTK && window.TOTK.state;
    if (!s || !s.selected) return false;
    var id = CAT_ID[currentLayer()];
    return !!(id && s.selected[id]);
  }

  function dragonById(id) {
    for (var i = 0; i < lastItems.length; i++) {
      if (lastItems[i] && lastItems[i].id === id) return lastItems[i];
    }
    return null;
  }

  var toastTimer = null;
  function toast(msg) {
    var el = document.getElementById('toast');
    if (!el) return;
    el.textContent = msg;
    el.classList.remove('hidden');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { el.classList.add('hidden'); }, 2400);
  }

  /* 右下角「移动到龙」按钮：hover 出四条龙，点击只移动视角、不跟随。
   * 菜单在按钮左侧展开；点击条目不收起，方便连续查看多条龙。 */
  function jumpToDragon(id) {
    var def = null;
    for (var i = 0; i < DRAGONS.length; i++) {
      if (DRAGONS[i].id === id) { def = DRAGONS[i]; break; }
    }
    var d = dragonById(id);
    if (!d || d.mx == null || d.my == null) {
      toast('尚未获取' + (def ? def.name : '龙') + '的位置（定位服务未启动或未锁定）');
      return;
    }
    var layer = Number(d.layer);
    if (layer && layer !== Number(currentLayer())) {
      if (window.TOTK && window.TOTK.switchLayer) window.TOTK.switchLayer(layer);
      /* 手动锁定目标层，避免实时自动切层立刻把视角拉回玩家所在层。 */
      if (window.LIVENAV && window.LIVENAV.lockLayer && window.LIVENAV.setLayerLock &&
          Number(window.LIVENAV.lockLayer()) !== layer) {
        window.LIVENAV.setLayerLock(layer);
      }
    }
    map.flyTo([d.mx, d.my], map.getZoom(), { duration: 0.8 });
    toast('已移动到 ' + (d.name || (def && def.name) || '龙') +
      (LAYER_NAME[layer] ? ' · ' + LAYER_NAME[layer] : ''));
  }

  function ensureMenu() {
    var bar = document.getElementById('zoomControls');
    if (!bar || document.getElementById('dragonJump')) return;
    var wrap = document.createElement('div');
    wrap.id = 'dragonJump';
    wrap.className = 'dj-wrap';

    var btn = document.createElement('button');
    btn.type = 'button';
    btn.id = 'dragonJumpBtn';
    btn.title = '移动到龙的位置（悬停展开，点击固定）';
    btn.textContent = '🐉';
    wrap.appendChild(btn);

    var menu = document.createElement('div');
    menu.id = 'dragonMenu';
    menu.className = 'dj-menu';
    var html = '<div class="dj-hint">移动到龙</div>';
    DRAGONS.forEach(function (d) {
      html += '<div class="dj-item off" data-id="' + d.id + '">' +
        '<span class="dj-dot" style="background:' + (COLORS[d.id] || '#fff') + '"></span>' +
        '<span class="dj-name">' + d.name + '</span>' +
        '<span class="dj-meta" data-meta="' + d.id + '">未获取</span>' +
      '</div>';
    });
    menu.innerHTML = html;
    wrap.appendChild(menu);

    var mirror = document.getElementById('btnMirror');
    bar.insertBefore(wrap, mirror || null);

    /* 悬停只做临时展开；点击按钮进入固定展开，菜单常驻，再点一次才收起。 */
    var pinned = false;
    btn.addEventListener('click', function (e) {
      e.preventDefault();
      e.stopPropagation();
      pinned = !pinned;
      wrap.classList.toggle('open', pinned);
      /* 取消固定时即使鼠标还停在按钮上也要立刻收起；离开再回来才会重新 hover 展开 */
      if (!pinned) wrap.classList.remove('hover');
    });
    if (window.matchMedia && window.matchMedia('(hover: hover)').matches) {
      wrap.addEventListener('mouseenter', function () { wrap.classList.add('hover'); });
      wrap.addEventListener('mouseleave', function () { wrap.classList.remove('hover'); });
    }
    menu.addEventListener('click', function (e) {
      var item = e.target && e.target.closest ? e.target.closest('.dj-item') : null;
      if (!item) return;
      e.stopPropagation();
      jumpToDragon(item.getAttribute('data-id'));
    });
    updateMenu();
  }

  function updateMenu() {
    var menu = document.getElementById('dragonMenu');
    if (!menu) return;
    DRAGONS.forEach(function (def) {
      var el = menu.querySelector('.dj-item[data-id="' + def.id + '"]');
      var meta = menu.querySelector('[data-meta="' + def.id + '"]');
      if (!el || !meta) return;
      var d = dragonById(def.id);
      var ok = !!(d && d.locked && d.updated && d.mx != null && d.my != null);
      el.classList.toggle('off', !ok);
      if (ok) {
        var layerName = LAYER_NAME[Number(d.layer)] || '';
        /* simulated=true = 龙在附近，游戏沿轨道实时更新坐标；false = 远处缓存（最后位置） */
        meta.textContent = (layerName ? layerName + ' · ' : '') + (d.simulated ? '实时' : '缓存');
      } else {
        meta.textContent = d ? '未锁定' : '未获取';
      }
    });
  }

  var ICONS = {
    light: 'dragon_light.png',
    dinraal: 'dragon_fire.png',
    naydra: 'dragon_ice.png',
    farosh: 'dragon_electric.png'
  };

  function injectCSS() {
    if (document.getElementById('dlg-live-css')) return;
    var st = document.createElement('style');
    st.id = 'dlg-live-css';
    st.textContent =
      '@keyframes dlg-live-pulse{' +
      '0%,100%{filter:drop-shadow(0 0 2px var(--glow)) drop-shadow(0 0 6px var(--glow)) drop-shadow(0 2px 5px rgba(0,0,0,.8));}' +
      '50%{filter:drop-shadow(0 0 8px var(--glow)) drop-shadow(0 0 18px var(--glow)) drop-shadow(0 2px 5px rgba(0,0,0,.8));}' +
      '}' +
      '.dlg-live-marker>div{animation:dlg-live-pulse 1.8s ease-in-out infinite;}' +
      '.dlg-live-marker.dlg-stale>div{animation:none;filter:drop-shadow(0 2px 5px rgba(0,0,0,.8));opacity:.55;}' +
      '.dj-wrap{position:relative;}' +
      '.dj-menu{display:none;position:absolute;right:calc(100% + 8px);bottom:0;min-width:180px;padding:6px;border-radius:10px;background:rgba(20,22,26,.96);border:1px solid rgba(255,255,255,.12);box-shadow:0 6px 22px rgba(0,0,0,.55);z-index:1200;}' +
      '.dj-menu::before{content:"";position:absolute;right:-8px;top:0;bottom:0;width:8px;}' +
      '.dj-wrap.open .dj-menu,.dj-wrap.hover .dj-menu{display:block;}' +
      '.dj-wrap.open #dragonJumpBtn{background:#ffe9a8;color:#6b4a00;border-color:#eac27e;}' +
      '.dj-hint{font-size:10.5px;color:#8b939d;padding:2px 8px 6px;user-select:none;}' +
      '.dj-item{display:flex;align-items:center;gap:8px;padding:7px 8px;border-radius:7px;cursor:pointer;font-size:12px;color:#dfe4ea;user-select:none;}' +
      '.dj-item:hover{background:rgba(234,194,126,.14);color:#fff;}' +
      '.dj-dot{width:8px;height:8px;border-radius:50%;flex:none;box-shadow:0 0 6px currentColor;}' +
      '.dj-name{flex:1;white-space:nowrap;}' +
      '.dj-meta{font-size:10.5px;color:#8b939d;font-variant-numeric:tabular-nums;}' +
      '.dj-item.off .dj-name,.dj-item.off .dj-meta{opacity:.45;}' +
      '.dj-item.off{cursor:default;}' +
      '.dj-item.active{background:rgba(234,194,126,.18);}';
    document.head.appendChild(st);
  }

  function iconFor(d) {
    var file = ICONS[d.id] || 'dragon_light.png';
    var color = COLORS[d.id] || '#ffffff';
    var filled = !!d.simulated;
    var size = filled ? 30 : 24;
    var cls = 'dlg-live-marker' + (filled ? '' : ' dlg-stale');
    var html = '<div style="width:' + size + 'px;height:' + size + 'px;--glow:' + color + ';">' +
      '<img src="assets/icons/' + file + '" width="' + size + '" height="' + size + '" ' +
      'style="display:block;pointer-events:none;"></div>';
    return L.divIcon({
      className: cls,
      html: html,
      iconSize: [size, size],
      iconAnchor: [size / 2, size / 2]
    });
  }

  function render(items) {
    lastItems = items || [];
    updateMenu();
    if (!map) return;
    if (group) { map.removeLayer(group); group = null; }

    /* 跟随「龙的轨迹」图层开关：关闭时实时图标也一并隐藏。 */
    if (!trailOn()) return;

    // 通知轨迹图层：有实时位置的龙隐藏静态头像，避免重复。
    var liveIds = {};
    lastItems.forEach(function (d) {
      if (d && d.locked && d.updated) liveIds[d.id] = true;
    });
    if (window.DragonLayer && window.DragonLayer.setLiveIds) {
      window.DragonLayer.setLiveIds(liveIds);
    }

    var layer = currentLayer();
    var g = L.layerGroup();
    var shown = 0;
    lastItems.forEach(function (d) {
      if (!d || !d.locked || !d.updated) return;
      if (Number(d.layer) !== Number(layer)) return;
      if (d.mx == null || d.my == null) return;
      var mk = L.marker([d.mx, d.my], { icon: iconFor(d), riseOnHover: true });
      mk.bindTooltip(
        d.name + (d.simulated ? '' : '（未在附近，最后位置）') +
        '<br>' + Number(d.gx).toFixed(1) + ', ' + Number(d.gy).toFixed(1) + ', ' + Number(d.gz).toFixed(1),
        { direction: 'top', offset: [0, -10], className: 'mk-label dlg-tip' }
      );
      mk.addTo(g);
      shown++;
    });
    if (shown > 0) { g.addTo(map); group = g; }
  }

  async function poll() {
    try {
      var r = await fetch(API + '/dragons?t=' + Date.now(), { cache: 'no-store' });
      if (!r.ok) return;
      var j = await r.json();
      if (j && j.ok) render(j.items);
    } catch (e) { /* 服务未启动时静默 */ }
  }

  /* 分类勾选 / 切层都不会触发事件，轮询兜底：开关或图层一变立刻重画，打开时立即取一帧。 */
  function observeState() {
    setInterval(function () {
      var on = trailOn();
      var layer = currentLayer();
      if (on === lastOn && layer === lastLayer) return;
      lastOn = on;
      lastLayer = layer;
      render(lastItems);
      if (on) poll();
    }, 200);
  }

  function init() {
    map = (window.TOTK && window.TOTK.map) || null;
    if (!map) { setTimeout(init, 300); return; }
    injectCSS();
    lastOn = trailOn();
    lastLayer = currentLayer();
    ensureMenu();
    observeState();
    poll();
    setInterval(poll, 1000);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
