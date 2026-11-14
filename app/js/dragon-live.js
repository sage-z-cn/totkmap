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
      '.dlg-live-marker.dlg-stale>div{animation:none;filter:drop-shadow(0 2px 5px rgba(0,0,0,.8));opacity:.55;}';
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
