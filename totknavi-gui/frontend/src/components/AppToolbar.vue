<template>
  <!-- 38px 工具条：左侧 Tab，右侧按当前页显示动作 -->
  <div class="h-[38px] shrink-0 flex items-center gap-2 px-3 bg-[#0d1117] border-b border-slate-800">
    <!-- Tab 切换；地图 Tab 带定位状态点 -->
    <div class="flex items-center gap-1 shrink-0">
      <button v-for="tab in TABS" :key="tab" @click="setTab(tab)"
        :class="activeTab === tab
          ? 'bg-slate-800 text-white border-slate-700'
          : 'text-slate-400 border-transparent hover:text-slate-200 hover:bg-slate-800/50'"
        class="h-[26px] px-2.5 rounded border text-[11px] font-medium flex items-center gap-1.5 transition">
        <span>{{ t(`tabs.${tab}`) }}</span>
        <span v-if="tab === 'map'" class="w-1.5 h-1.5 rounded-full transition-colors" :class="mapDot"></span>
      </button>
    </div>

    <span class="w-px h-4 bg-slate-800 shrink-0"></span>

    <div class="ml-auto flex items-center gap-1.5 shrink-0">
      <!-- 控制台：平台选择 + 打开地图 + 开始/停止定位 -->
      <template v-if="activeTab === 'control'">
        <select v-model="cfg.emu" @change="onEmuChange" :title="t('emu.pickTitle')"
          class="h-[26px] bg-[#161b22] border border-slate-700 rounded px-1.5 text-[11px] font-medium text-slate-300 hover:border-slate-500 focus:outline-none transition">
          <option value="auto">{{ t('emu.auto') }}</option>
          <option value="ryujinx">{{ t('emu.ryujinx') }}</option>
          <option value="eden">{{ t('emu.eden') }}</option>
        </select>
        <button @click="launchGame" :disabled="gameRunning"
          :class="gameRunning
            ? 'bg-slate-800/60 border-slate-700 text-emerald-400 cursor-default'
            : 'bg-emerald-600 hover:bg-emerald-500 border-emerald-500 text-white'"
          class="h-[26px] px-2.5 rounded border text-[11px] font-bold flex items-center gap-1.5 transition active:scale-95 disabled:opacity-100">
          <span>{{ gameRunning ? '●' : '▶' }}</span>
          <span>{{ gameRunning ? t('toolbar.gameRunning') : t('toolbar.launchGame') }}</span>
        </button>
        <button @click="setTab('map')"
          class="h-[26px] px-2.5 rounded border border-slate-700 bg-slate-800 hover:bg-slate-700 text-sky-400 text-[11px] font-medium flex items-center gap-1.5 transition">
          <span>🧭</span>
          <span>{{ t('buttons.openMap') }}</span>
        </button>
        <button @click="toggleLocating"
          :class="coreRunning
            ? 'bg-emerald-600 hover:bg-emerald-500 shadow-emerald-950/40'
            : 'bg-blue-600 hover:bg-blue-500 shadow-blue-950/40'"
          class="h-[26px] px-3 rounded text-white text-[11px] font-bold flex items-center gap-1.5 transition shadow active:scale-95">
          <span>{{ coreRunning ? '◉' : '▶' }}</span>
          <span>{{ coreRunning ? t('buttons.locating') : t('buttons.start') }}</span>
        </button>
      </template>

      <!-- 地图：来源徽标 + 刷新 + 浏览器打开 -->
      <template v-else>
        <span class="h-[26px] px-2 rounded border text-[10px] font-mono flex items-center gap-1.5" :class="badgeClass">
          <span class="w-1.5 h-1.5 rounded-full" :class="badgeDot"></span>
          <span>{{ badgeText }}</span>
        </span>
        <button @click="refreshMap"
          class="h-[26px] px-2.5 rounded border border-slate-700 bg-slate-800 hover:bg-slate-700 text-slate-200 text-[11px] flex items-center gap-1.5 transition active:scale-95">
          <span>↻</span>
          <span>{{ t('toolbar.refresh') }}</span>
        </button>
        <button @click="openMap"
          class="h-[26px] px-2.5 rounded border border-slate-700 bg-slate-800 hover:bg-slate-700 text-sky-400 text-[11px] flex items-center gap-1.5 transition active:scale-95">
          <span>↗</span>
          <span>{{ t('toolbar.openBrowser') }}</span>
        </button>
      </template>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  cfg, activeTab, setTab, coreRunning, locked, mapMode, refreshMap,
  openMap, toggleLocating, onEmuChange, gameRunning, launchGame
} from '../composables/useCore'

const { t } = useI18n()

const TABS = ['control', 'map']

// 地图 Tab 状态点：绿=已锁定 / 黄=核心在跑未锁定 / 灰=核心未运行
const mapDot = computed(() => {
  if (locked.value) return 'bg-emerald-400'
  if (coreRunning.value) return 'bg-amber-400 animate-pulse'
  return 'bg-slate-600'
})

const badgeText = computed(() => {
  return mapMode.value === 'offline' ? t('toolbar.mapOffline') : t('toolbar.mapOnline')
})
const badgeClass = computed(() => {
  if (mapMode.value === 'offline') return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'
  return 'border-blue-500/30 bg-blue-500/10 text-sky-300'
})
const badgeDot = computed(() => mapMode.value === 'offline' ? 'bg-emerald-400' : 'bg-sky-400')
</script>
