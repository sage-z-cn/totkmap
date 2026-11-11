<template>
  <!-- 36px 自绘标题栏：整栏可拖拽，按钮区 no-drag -->
  <header class="h-9 shrink-0 flex items-center gap-2 px-3 bg-[#161b22] border-b border-slate-800" style="--wails-draggable:drag;">
    <div class="flex items-center gap-2 min-w-0 shrink-0">
      <span class="w-2 h-2 rounded-full shrink-0" :class="syncDot"></span>
      <span class="text-[12px] font-bold tracking-wide text-white truncate">{{ t('title.main') }}</span>
      <span v-if="ver" class="text-[10px] font-mono leading-none text-sky-300 bg-blue-500/10 border border-blue-500/40 rounded px-1.5 py-1 shrink-0">v{{ ver.replace('v', '') }}</span>
    </div>

    <div class="ml-auto flex items-center gap-1 shrink-0" style="--wails-draggable:no-drag;">
      <span class="text-[10px] font-mono mr-1.5 max-w-[220px] truncate" :class="syncClass">{{ syncText }}</span>

      <!-- 设置卡片（点击齿轮弹出，与关于互斥） -->
      <div class="relative" @click.stop>
        <button class="w-7 h-7 flex items-center justify-center rounded text-slate-400 hover:text-white hover:bg-slate-700/60 text-[13px] leading-none transition" :title="t('header.settings')" @click="toggleSettings">⚙</button>
        <SettingsPop v-if="showSettingsPop" class="absolute right-0 top-full mt-1.5 z-50" />
      </div>
      <!-- 关于卡片（点击问号弹出，与设置互斥） -->
      <div class="relative" @click.stop>
        <button class="w-7 h-7 flex items-center justify-center rounded text-slate-400 hover:text-white hover:bg-slate-700/60 text-[13px] font-semibold leading-none transition" :title="t('header.about')" @click="toggleAbout">?</button>
        <AboutPop v-if="showAboutPop" class="absolute right-0 top-full mt-1.5 z-50" />
      </div>

      <span class="w-px h-3.5 bg-slate-700/70 mx-0.5"></span>

      <button class="w-7 h-7 flex items-center justify-center rounded text-slate-400 hover:text-white hover:bg-slate-700/60 text-[13px] leading-none transition" :title="t('header.minimize')" @click="minWin">—</button>
      <button class="w-7 h-7 flex items-center justify-center rounded text-slate-400 hover:text-white hover:bg-slate-700/60 text-[12px] leading-none transition" :title="windowMaximised ? t('header.restore') : t('header.maximize')" @click="toggleMax">{{ windowMaximised ? '❐' : '⛶' }}</button>
      <button class="w-7 h-7 flex items-center justify-center rounded text-slate-400 hover:text-rose-400 hover:bg-rose-500/15 text-[12px] leading-none transition" :title="t('header.close')" @click="closeWin">✕</button>
    </div>
  </header>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import SettingsPop from './SettingsPop.vue'
import AboutPop from './AboutPop.vue'
import { rt, ver, syncDot, syncText, syncClass, windowMaximised, syncWindowMaximised } from '../composables/useCore'

const { t } = useI18n()

// ---- 窗口控制 ----
// 最大化状态放在 useCore：定位后自动移屏/最大化时 TitleBar 也要同步图标
async function toggleMax() {
  const prev = windowMaximised.value
  rt.WindowToggleMaximise()
  if (!rt.WindowIsMaximised) { windowMaximised.value = !prev; return }
  await syncWindowMaximised()
  setTimeout(syncWindowMaximised, 150) // 少数情况下窗口状态落定略晚，再对一次图标
}
function minWin() { rt.WindowMinimise() }
// Wails 无边框(Frameless)下 WindowClose() 不触发关闭，改用 Quit() 强制退出
function closeWin() { rt.Quit() }

// 设置/关于卡片：互斥显示（同时只弹一个），点击外部全部关闭
const showSettingsPop = ref(false)
const showAboutPop = ref(false)
function toggleSettings() {
  showAboutPop.value = false
  showSettingsPop.value = !showSettingsPop.value
}
function toggleAbout() {
  showSettingsPop.value = false
  showAboutPop.value = !showAboutPop.value
}
function closePops() {
  showSettingsPop.value = false
  showAboutPop.value = false
}

onMounted(() => {
  syncWindowMaximised()
  document.addEventListener('click', closePops)
})
onUnmounted(() => document.removeEventListener('click', closePops))
</script>
