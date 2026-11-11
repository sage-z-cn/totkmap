<template>
  <div class="flex-1 flex flex-col gap-2.5 min-h-0 p-3 overflow-y-auto">

    <!-- 紧凑步骤条：圆点 + 文字，无卡片背景（高度 22px） -->
    <div class="flex items-center gap-1.5 text-[11px] font-mono shrink-0 h-[22px] px-0.5 whitespace-nowrap overflow-hidden">
      <template v-for="(s, i) in steps" :key="s.key">
        <div v-if="i > 0" class="text-slate-700 mx-0.5">›</div>
        <div class="flex items-center gap-1.5" :class="stepClass(s.state)">
          <span class="w-1.5 h-1.5 rounded-full" :class="stepDot(s.state)"></span>
          <span>{{ t(`steps.${s.key}`) }}</span>
        </div>
      </template>
    </div>

    <!-- ② 两列：左4 实时数据 ｜ 右6 本地解析进度 -->
    <div class="grid grid-cols-10 gap-3 items-stretch shrink-0">

      <!-- 左 4 列 -->
      <div class="col-span-4 flex flex-col gap-3">

        <!-- 实时同步坐标卡 -->
        <section class="bg-[#161b22] border border-slate-800 rounded-xl p-3.5 flex-1">
          <div class="flex items-center justify-between text-xs pb-2 border-b border-slate-800/80 mb-3">
            <span class="font-bold text-sky-400 flex items-center gap-1.5">{{ t('locate.liveCoords') }}</span>
            <span class="text-[10px] font-mono text-slate-500">{{ t('locate.offset') }}: {{ lockAddrText }}</span>
          </div>
          <div class="grid grid-cols-3 gap-2 text-center font-mono">
            <div class="bg-[#0d1117] p-2 rounded-lg border border-slate-800">
              <div class="text-[10px] text-slate-500">X</div>
              <div class="text-sm font-bold text-white mt-0.5">{{ fmt(pos.gx) }}</div>
            </div>
            <div class="bg-[#0d1117] p-2 rounded-lg border border-slate-800">
              <div class="text-[10px] text-slate-500">Y</div>
              <div class="text-sm font-bold text-white mt-0.5">{{ fmt(pos.gy) }}</div>
            </div>
            <div class="bg-[#0d1117] p-2 rounded-lg border border-slate-800">
              <div class="text-[10px] text-slate-500">{{ t('locate.axisZ') }}</div>
              <div class="text-sm font-bold text-white mt-0.5">{{ fmt(pos.gz) }}</div>
            </div>
          </div>
          <div class="mt-2.5 flex items-center justify-between text-xs bg-[#0d1117] px-3 py-2 rounded-lg border border-slate-800">
            <span class="text-slate-400">{{ t('locate.currentLayer') }}:</span>
            <span class="font-bold text-emerald-400">{{ regionText }}<span class="text-blue-300 text-[10px] ml-2">{{ sourceLabel }}</span></span>
          </div>
        </section>

        <!-- 坐标校准卡（V1.4.0：无法定位/秒锁失败时，GUI 提交游戏 HUD 坐标精确匹配） -->
        <section class="bg-[#161b22] border border-slate-800 rounded-xl p-3.5 shrink-0">
          <div class="flex items-center justify-between text-xs pb-2 border-b border-slate-800/80 mb-2.5">
            <span class="font-bold text-emerald-400 flex items-center gap-1.5">🧭 {{ t('locate.calibration') }}</span>
            <div class="flex items-center gap-2">
              <span class="text-[10px] text-slate-500">{{ t('locate.calWhenFail') }}</span>
              <button @click="doCalibrate" :disabled="calBusy"
                class="bg-emerald-600 hover:bg-emerald-500 text-white px-2.5 py-1 rounded-md text-[10px] font-bold transition flex items-center gap-1 active:scale-95 disabled:opacity-60 disabled:cursor-wait">
                <span>↻</span>
                <span>{{ t('locate.submit') }}</span>
              </button>
            </div>
          </div>
          <div class="text-[10px] text-slate-500 mb-2.5 leading-relaxed">{{ t('locate.calHint') }}</div>
          <div class="flex items-center gap-2">
            <label class="flex-1 bg-[#0d1117] rounded-lg border border-slate-800 px-2 py-1.5 flex items-center gap-1.5">
              <span class="text-[10px] text-slate-500 shrink-0">X</span>
              <input v-model="calX" type="number" step="0.1" :placeholder="t('locate.placeholder', { v: '-298.8' })"
                class="bg-transparent outline-none text-white font-mono text-xs w-full placeholder:text-slate-600" />
            </label>
            <label class="flex-1 bg-[#0d1117] rounded-lg border border-slate-800 px-2 py-1.5 flex items-center gap-1.5">
              <span class="text-[10px] text-slate-500 shrink-0">Y</span>
              <input v-model="calY" type="number" step="0.1" :placeholder="t('locate.placeholder', { v: '142.8' })"
                class="bg-transparent outline-none text-white font-mono text-xs w-full placeholder:text-slate-600" />
            </label>
            <label class="flex-1 bg-[#0d1117] rounded-lg border border-slate-800 px-2 py-1.5 flex items-center gap-1.5">
              <span class="text-[10px] text-slate-500 shrink-0">Z</span>
              <input v-model="calZ" type="number" step="0.1" :placeholder="t('locate.placeholder', { v: '25.0' })"
                class="bg-transparent outline-none text-white font-mono text-xs w-full placeholder:text-slate-600" />
            </label>
          </div>
          <!-- 预设常用地标 -->
          <div class="flex items-center gap-1.5 mt-2.5 text-[10px] flex-wrap">
            <span class="text-slate-500 shrink-0">{{ t('locate.preset') }}:</span>
            <button @click="fillPreset(-298.8, 142.8, 25.0)" class="px-1.5 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition" :title="t('locate.tower') + ' (-298.8, 142.8, 25.0)'">{{ t('locate.tower') }}</button>
            <button @click="fillPreset(-257.5, 105.0, 20.0)" class="px-1.5 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition" :title="t('locate.landing') + ' (-257.5, 105.0, 20.0)'">{{ t('locate.landing') }}</button>
            <button @click="fillPreset(-204.3, 451.0, 21.0)" class="px-1.5 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition" :title="t('locate.northShrine') + ' (巧诺尼希乌神庙: -204.3, 451.0, 21.0)'">{{ t('locate.northShrine') }}</button>
            <button @click="fillPreset(453.0, -774.0, 1466.0)" class="px-1.5 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition" :title="t('locate.awakeningRoom') + ' (初始空岛: 453.0, -774.0, 1466.0)'">{{ t('locate.awakeningRoom') }}</button>
          </div>
        </section>
      </div>

      <!-- 右 6 列：本地解析进度（全量 21 类 + 紧凑） -->
      <div class="col-span-6 flex flex-col min-h-0">
        <section class="bg-[#161b22] border border-slate-800 rounded-xl p-3 flex-1 flex flex-col min-h-0 justify-between">
          <div>
            <div class="flex items-center justify-between text-xs pb-1.5 border-b border-slate-800/80 shrink-0">
              <span class="font-bold text-emerald-400">{{ t('locate.localProgress') }}</span>
              <span class="text-[10px] text-slate-500">{{ t('locate.categoryCount', { n: progressCount }) }}</span>
            </div>
            <div v-if="progressData.counts" class="mt-1.5">
              <div class="grid grid-cols-3 gap-x-1.5 gap-y-1">
                <div v-for="c in allCategories" :key="c.key"
                  class="bg-[#0d1117] py-1 px-2 rounded border border-slate-800 flex justify-between items-center text-[12px]">
                  <span class="text-slate-400 truncate">{{ c.label }}</span>
                  <span class="font-mono text-white font-semibold shrink-0 ml-1">{{ n(c.key) }}</span>
                </div>
              </div>
            </div>
            <div v-else class="flex items-center justify-center text-[11px] text-slate-500 my-4">{{ t('locate.waitingProgress') }}</div>
          </div>

          <!-- 生效存档管理条：紧凑布局 -->
          <div class="mt-2 pt-1.5 border-t border-slate-800/80 text-xs shrink-0">
            <div class="flex items-center justify-between gap-1.5 flex-wrap">
              <!-- 四个动作按钮 -->
              <div class="flex items-center gap-1.5">
                <button @click="pickSaveFile" class="bg-blue-600 hover:bg-blue-500 text-white px-2 py-1 rounded text-[11px] font-medium transition active:scale-95">
                  {{ t('buttons.chooseFile') }}
                </button>
                <button @click="pickSaveDir" class="bg-slate-700 hover:bg-slate-600 text-slate-200 px-2 py-1 rounded text-[11px] transition active:scale-95">
                  {{ t('buttons.chooseDir') }}
                </button>
                <button v-if="progressData.save" @click="doOpenSave" class="bg-slate-700 hover:bg-slate-600 text-slate-200 px-2 py-1 rounded text-[11px] transition active:scale-95">
                  {{ t('buttons.locateFile') }}
                </button>
                <button @click="doResetDetection" :class="resetSuccess ? 'bg-emerald-600 text-white font-semibold' : 'bg-amber-600/80 hover:bg-amber-600 text-white'" class="px-2 py-1 rounded text-[11px] transition active:scale-95">
                  {{ resetSuccess ? '✓ 已重置' : t('buttons.detectSave') }}
                </button>
              </div>

              <!-- 右侧：最新存档相对时间提醒 -->
              <div class="flex items-center gap-2 ml-auto text-[11px]">
                <div v-if="saveTimeAgoText" class="font-mono text-[11px] shrink-0" :class="saveTimeAgoClass">
                  {{ t('locate.latestSaveTime') }}: {{ saveTimeAgoText }}
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2 mt-2">
              <span class="text-[11px] text-slate-400 font-medium shrink-0">{{ t('locate.saveInfo') }}:</span>
              <span class="text-[10px] font-mono text-slate-300 truncate flex-1" :title="progressData.save || t('locate.noSave')">
                {{ progressData.save || t('locate.noSave') }}
              </span>
            </div>
          </div>
        </section>
      </div>
    </div>

    <!-- ③ 实时日志 -->
    <LogPanel />

    <!-- ④ 坐标校准预检弹窗（v2.1.2 B-1：>50m 警告，可强制继续） -->
    <div v-if="showPrecheck && precheckData" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50" style="--wails-draggable:no-drag;" @click.self="showPrecheck = false">
      <div class="bg-[#161b22] border border-amber-500/50 rounded-xl p-4 shadow-2xl text-xs space-y-3 w-[400px]">
        <div class="text-[11px] font-semibold text-amber-400 uppercase tracking-wider">{{ t('precheck.title') }}</div>
        <div class="text-slate-300">{{ t('precheck.descPre') }}<span class="font-mono text-amber-400">{{ precheckData.d.toFixed(0) }}m</span></div>
        <div class="font-mono text-[11px] space-y-1">
          <div class="text-slate-500">{{ t('precheck.currentLock') }}：<span class="text-slate-300">({{ precheckData.cx.toFixed(1) }}, {{ precheckData.cy.toFixed(1) }}, {{ precheckData.cz.toFixed(1) }})</span></div>
          <div class="text-slate-500">{{ t('precheck.yourInput') }}：<span class="text-white">({{ precheckData.x }}, {{ precheckData.y }}, {{ precheckData.z }})</span></div>
        </div>
        <div class="text-[10px] text-slate-500 leading-relaxed">
          {{ t('precheck.tip1') }}<br>
          {{ t('precheck.tip2') }}<br>
          {{ t('precheck.tip3') }}
        </div>
        <div class="flex justify-end gap-2 pt-1">
          <button class="bg-slate-700 hover:bg-slate-600 px-3 py-1.5 rounded text-xs text-slate-200" @click="showPrecheck = false">{{ t('buttons.cancel') }}</button>
          <button class="bg-amber-600 hover:bg-amber-500 px-3 py-1.5 rounded text-xs text-white font-semibold" @click="realSubmit(precheckData.x, precheckData.y, precheckData.z)">{{ t('buttons.confirm') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LogPanel from '../components/LogPanel.vue'
import { findRegion } from '../data/regions'
import { PROGRESS_CATEGORY_KEYS, progressLabel } from '../data/progressCategories'
import {
  pos, steps, stepClass, stepDot, progressData,
  layerName, sourceLabel, logs,
  submitCoords, coordsState,
  coordsErrorText, saveTimeAgoText, saveTimeAgoClass,
  pickSaveFile, pickSaveDir, doOpenSave, resetSaveDetection
} from '../composables/useCore'

const { t } = useI18n()

// 全量 21 类（与 core progressCounts 对齐）：key 是 core 中文类名（数据层），
// label 走 i18n（英文界面显示官方英文译名，见 data/progressCategories.js）
const allCategories = computed(() =>
  PROGRESS_CATEGORY_KEYS.map(key => ({ key, label: progressLabel(key) }))
)

function fmt(v) { return (!pos.ok || v == null) ? '-' : Number(v).toFixed(1) }
function n(key) {
  const c = progressData.value?.counts?.[key]
  if (!c) return '-'
  return `${c.done}/${c.total}`
}
const progressCount = computed(() => {
  const counts = progressData.value?.counts
  return counts ? Object.keys(counts).filter(k => counts[k] && counts[k].total).length : 0
})

const resetSuccess = ref(false)
async function doResetDetection() {
  await resetSaveDetection()
  resetSuccess.value = true
  const tNow = new Date().toTimeString().slice(0, 8)
  logs.value.push({ time: tNow, text: t("locate.resetAutoDetectTip"), level: "PROGRESS", color: "text-emerald-300", levelColor: "text-emerald-400 font-semibold" })
  if (logs.value.length > 2000) logs.value = logs.value.slice(logs.value.length - 2000)
  setTimeout(() => { resetSuccess.value = false }, 1500)
}

const lockAddrText = computed(() => (pos.lockAddr && pos.lockAddr !== '0x0' ? pos.lockAddr : '—'))

// ---- 坐标校准状态 ----
const calX = ref('')
const calY = ref('')
const calZ = ref('')
const calBusy = ref(false)

function fillPreset(x, y, z) {
  calX.value = x
  calY.value = y
  calZ.value = z
}
// 校准结果写入实时日志（复用 logs 共享 ref）：失败红色 / 成功绿色，不占按钮行空间
function pushCalLog(text, ok) {
  const t = new Date().toTimeString().slice(0, 8)
  logs.value.push(ok
    ? { time: t, text, level: 'ok', color: 'text-emerald-400', levelColor: 'text-emerald-500' }
    : { time: t, text, level: 'error', color: 'text-red-400', levelColor: 'text-red-500' })
  if (logs.value.length > 2000) logs.value = logs.value.slice(logs.value.length - 2000)
}
// v2.1.2 B-1 预检：填入坐标与当前锁定位置差 >50m 时弹 Vue 悬浮卡警告
//（Wails WebView2 禁用原生 confirm/alert，必须用组件；AboutPop 同款样式）。
// 可强制继续——当前锁定位置本身可能锁错（Bug C 场景），基准不可信。
const showPrecheck = ref(false)
const precheckData = ref(null)
async function doCalibrate() {
  const x = Number(calX.value), y = Number(calY.value), z = Number(calZ.value)
  if (isNaN(x) || isNaN(y) || isNaN(z) || (x === 0 && y === 0 && z === 0)) {
    pushCalLog(t('cal.needInput'), false)
    return
  }
  if (pos.ok && pos.gx != null) {
    const d = Math.sqrt((x - pos.gx) ** 2 + (y - pos.gy) ** 2 + (z - pos.gz) ** 2)
    if (d > 50) {
      precheckData.value = { x, y, z, cx: pos.gx, cy: pos.gy, cz: pos.gz, d }
      showPrecheck.value = true
      return // 停在这里等用户选择
    }
  }
  await realSubmit(x, y, z)
}
async function realSubmit(x, y, z) {
  showPrecheck.value = false
  precheckData.value = null
  calBusy.value = true
  pushCalLog(t('cal.submitting'), false)
  await submitCoords(x, y, z)
  // 轮询结果：coordsState.result 由 pollCoordsResult 更新（搜索约 20-30s）
  const iv = setInterval(() => {
    if (!coordsState.busy) {
      clearInterval(iv)
      calBusy.value = false
      const r = coordsState.result
      if (!r || !r.ok) {
        // Go 双语错误按当前语言取（errorEn 优先）；前端兜底文案走 i18n
        pushCalLog(t('cal.failed', { err: coordsErrorText(r) }), false)
      } else {
        const cp = r.copies ? t('cal.copies', { n: r.copies }) : ''
        pushCalLog(t('cal.locked', { addr: r.addr, cp }), true)
      }
    }
  }, 1000)
}

// 所在图层行：地面层显示地区名（TOTKmap areas.js 最近区域匹配），地下/天空仅层名
const regionText = computed(() => {
  if (!pos.ok) return '—'
  const r = findRegion(pos.mx, pos.my, pos.layer)
  return r ? `${r.name} · ${layerName.value} (${pos.layer})` : `${layerName.value} (${pos.layer})`
})

</script>
