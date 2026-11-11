<template>
  <div class="flex-1 min-h-0 relative bg-[#0d1117]">
    <!-- 地图 iframe：v-show 常驻挂载（切 Tab 不重载），改 src 才刷新 -->
    <iframe
      v-show="hasFrame"
      :src="frameSrc"
      title="TOTKNavi Map"
      referrerpolicy="no-referrer"
      class="absolute inset-0 w-full h-full border-0 bg-[#0d1117]"
    ></iframe>

    <!-- 定位服务未启动 -->
    <div v-if="!coreRunning" class="absolute inset-0 flex flex-col items-center justify-center gap-3 px-8 text-center">
      <div class="text-3xl opacity-30">🧭</div>
      <div class="text-sm font-semibold text-slate-300">{{ t('map.notRunning') }}</div>
      <div class="text-[11px] text-slate-500 leading-relaxed max-w-[400px]">{{ t('map.notRunningHint') }}</div>
      <button @click="toggleLocating"
        class="mt-1 h-[30px] px-4 rounded bg-blue-600 hover:bg-blue-500 text-white text-[11px] font-bold flex items-center gap-1.5 transition shadow-lg shadow-blue-950/40 active:scale-95">
        <span>▶</span>
        <span>{{ t('map.startService') }}</span>
      </button>
    </div>

    <!-- 8766 端口被占用：内嵌地图拿不到 core 的 web 端（操作在顶部工具条，此处不重复放按钮） -->
    <div v-else-if="!webOK" class="absolute inset-0 flex flex-col items-center justify-center gap-3 px-8 text-center">
      <div class="text-3xl opacity-30">⚠</div>
      <div class="text-sm font-semibold text-amber-400">{{ t('map.portBusy') }}</div>
      <div class="text-[11px] text-slate-500 leading-relaxed max-w-[420px]">{{ t('map.portBusyHint') }}</div>
    </div>
    <!-- 核心已启动但 8766 还没 listen：等探测通过再挂 iframe -->
    <div v-else-if="!mapServerReady" class="absolute inset-0 flex flex-col items-center justify-center gap-3 px-8 text-center">
      <div class="text-3xl opacity-30 animate-pulse">🧭</div>
      <div class="text-sm font-semibold text-slate-300">{{ t('map.starting') }}</div>
      <div class="text-[11px] text-slate-500 leading-relaxed max-w-[420px]">{{ t('map.startingHint') }}</div>
      <button @click="refreshMap"
        class="mt-1 h-[28px] px-3 rounded bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-200 text-[11px] flex items-center gap-1.5 transition active:scale-95">
        <span>↻</span>
        <span>{{ t('map.retry') }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  coreRunning, webOK, mapSrc, mapMode, mapRefreshTick, mapServerReady,
  toggleLocating, refreshMap
} from '../composables/useCore'

const { t } = useI18n()

// 有地图源且 8766 已探测通过才挂 iframe；无源时保持为空，避免 src="" 让 webview 重载自身
const hasFrame = computed(() => mapServerReady.value && mapMode.value !== 'none' && !!mapSrc.value)

// 跨源 iframe 拿不到 contentWindow，刷新只能靠换 src + 时间戳
const frameSrc = computed(() => {
  const base = mapSrc.value
  if (!base) return null
  const sep = base.includes('?') ? '&' : '?'
  return `${base}${sep}_r=${mapRefreshTick.value}`
})
</script>
