/* ============================================================
 * 龙实时位置图层（dragon-live.js）
 * ------------------------------------------------------------
 * 数据：totknavi / totknavi-pc 的 GET /dragons
 *   { ok, items:[{id,name,gx,gy,gz,mx,my,layer,simulated,locked,updated}] }
 * 渲染：按当前地图层（18 地上 / 19 地下 / 20 天空）显示已锁定且同层的龙；
 *       simulated=false 时用空心圈表示"该龙不在附近、坐标为最后缓存"。
 * 开关：地图右上角「龙」按钮（localStorage 记忆）。
 * 说明：龙的飞行轨迹由已有的 dragon-layer.js 负责（侧栏「龙的轨迹」分类），
 *       本模块只画实时位置标记，不重复画线。
 * ============================================================ */
(function () {
  'use strict';

  var LS_KEY = 'totkmap.dragonLive.on.v1';
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
  var enabled = localStorage.getItem(LS_KEY) !== '0';
  var lastItems = [];
  var btn = null;

  function currentLayer() {
    if (window.TOTK && window.TOTK.state && window.TOTK.state.layer != null) {
      return window.TOTK.state.layer;
    }
    var active = document.querySelector('#layerSwitch button.active');
    return active ? Number(active.getAttribute('data-layer')) : 18;
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
      '0%,100%{transform:scale(1);filter:drop-shadow(0 0 2px var(--glow)) drop-shadow(0 0 7px var(--glow)) drop-shadow(0 2px 5px rgba(0,0,0,.8));}' +
      '50%{transform:scale(1.18);filter:drop-shadow(0 0 9px var(--glow)) drop-shadow(0 0 22px var(--glow)) drop-shadow(0 2px 5px rgba(0,0,0,.8));}' +
      '}' +
      '.dlg-live-marker>div{animation:dlg-live-pulse 1.8s ease-in-out infinite;transform-origin:center center;}' +
      '.dlg-live-marker.dlg-stale>div{animation:none;transform:none;filter:drop-shadow(0 2px 5px rgba(0,0,0,.8));opacity:.55;}';
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

    // 通知轨迹图层：有实时位置的龙隐藏静态头像，避免重复。
    var liveIds = {};
    lastItems.forEach(function (d) {
      if (d && d.locked && d.updated) liveIds[d.id] = true;
    });
    if (window.DragonLayer && window.DragonLayer.setLiveIds) {
      window.DragonLayer.setLiveIds(liveIds);
    }

    if (!enabled) return;

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

  function startCalibrate() {
    var pick = prompt('校准哪条龙？\n1 = 白龙\n2 = 奥尔龙\n3 = 聂尔龙\n4 = 费罗龙');
    var ids = { '1': 'light', '2': 'dinraal', '3': 'naydra', '4': 'farosh' };
    var did = ids[pick];
    if (!did) return;
    alert('请在地图上点击龙当前所在位置');
    map.once('click', function (e) {
      var url = API + '/dragons/calibrate?id=' + did +
        '&mx=' + e.latlng.lat + '&my=' + e.latlng.lng;
      fetch(url, { cache: 'no-store' })
        .then(function (r) { return r.json(); })
        .then(function (j) {
          if (j && j.ok) { poll(); }
          else { alert('校准失败：' + (j && j.error || '未知错误')); }
        })
        .catch(function () { alert('校准请求失败'); });
    });
  }

  function ensureButton() {
    if (btn || !map) return;
    var Ctl = L.Control.extend({
      options: { position: 'topright' },
      onAdd: function () {
        var div = L.DomUtil.create('div', 'leaflet-bar');
        btn = L.DomUtil.create('a', '', div);
        btn.href = '#';
        btn.title = '龙实时位置';
        btn.innerHTML = '龙';
        btn.style.cssText = 'display:flex;align-items:center;justify-content:center;width:30px;height:30px;' +
          'font:600 14px/1 system-ui,sans-serif;text-decoration:none;color:#333;background:#fff;';
        L.DomEvent.on(btn, 'click', function (e) {
          L.DomEvent.preventDefault(e);
          enabled = !enabled;
          localStorage.setItem(LS_KEY, enabled ? '1' : '0');
          updateBtn();
          render(lastItems);
        });
        var cal = L.DomUtil.create('a', '', div);
        cal.href = '#';
        cal.title = '校准龙位置（点击地图上龙的位置）';
        cal.innerHTML = '校';
        cal.style.cssText = 'display:flex;align-items:center;justify-content:center;width:30px;height:30px;' +
          'font:600 13px/1 system-ui,sans-serif;text-decoration:none;color:#555;background:#fff;' +
          'border-top:1px solid #ddd;';
        L.DomEvent.on(cal, 'click', function (e) {
          L.DomEvent.preventDefault(e);
          startCalibrate();
        });
        return div;
      }
    });
    map.addControl(new Ctl());
    updateBtn();
  }

  function updateBtn() {
    if (!btn) return;
    btn.style.background = enabled ? '#ffe9a8' : '#fff';
    btn.style.color = enabled ? '#6b4a00' : '#888';
  }

  function init() {
    map = (window.TOTK && window.TOTK.map) || null;
    if (!map) { setTimeout(init, 300); return; }
    injectCSS();
    ensureButton();
    poll();
    setInterval(poll, 1000);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
