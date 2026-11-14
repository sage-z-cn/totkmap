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

    <!-- 只有“本机无内置地图、改走核心 8766”时才需要等端口 listen；GUI 自带托管随时可看 -->
    <div v-if="initDone && !hasFrame" class="absolute inset-0 flex flex-col items-center justify-center gap-3 px-8 text-center">
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
  initDone, mapSrc, mapRefreshTick, mapServerReady, refreshMap
} from '../composables/useCore'

const { t } = useI18n()

// 地图源就绪才挂 iframe：内置托管/在线站点直接可用，核心 8766 等探测通过
const hasFrame = computed(() => initDone.value && mapServerReady.value && !!mapSrc.value)

// 跨源 iframe 拿不到 contentWindow，刷新只能靠换 src + 时间戳
const frameSrc = computed(() => {
  const base = mapSrc.value
  if (!base) return null
  const sep = base.includes('?') ? '&' : '?'
  return `${base}${sep}_r=${mapRefreshTick.value}`
})
</script>
