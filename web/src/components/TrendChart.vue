<template>
  <div ref="chartRef" :style="{ height: height, width: '100%' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, shallowRef } from 'vue'
import type { EChartsType } from '../utils/echarts'

interface SeriesItem {
  name: string
  data: number[]
  type?: 'line' | 'bar'
}

const props = withDefaults(defineProps<{
  dates: string[]
  series: SeriesItem[]
  title?: string
  yAxisName?: string
  height?: string
}>(), {
  height: '320px',
  yAxisName: '人数',
})

const chartRef = ref<HTMLElement>()
const chart = shallowRef<EChartsType>()
let echartsModule: typeof import('../utils/echarts') | undefined

async function loadECharts() {
  if (!echartsModule) {
    echartsModule = await import('../utils/echarts')
  }
  return echartsModule
}

function buildOption() {
  return {
    title: props.title ? { text: props.title, left: 'center', textStyle: { fontSize: 14 } } : undefined,
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0 },
    grid: { left: '3%', right: '4%', bottom: '12%', top: props.title ? '15%' : '8%', containLabel: true },
    xAxis: { type: 'category', data: props.dates, boundaryGap: false },
    yAxis: { type: 'value', name: props.yAxisName },
    series: props.series.map((s) => ({
      name: s.name,
      type: s.type || 'line',
      data: s.data,
      smooth: true,
      areaStyle: s.type !== 'bar' ? { opacity: 0.15 } : undefined,
    })),
  }
}

async function renderChart() {
  if (!chartRef.value) return
  const echarts = await loadECharts()
  if (!chartRef.value) return
  if (!chart.value) {
    chart.value = echarts.init(chartRef.value)
  }
  chart.value.setOption(buildOption(), true)
}

function handleResize() {
  chart.value?.resize()
}

onMounted(() => {
  renderChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  chart.value?.dispose()
})

watch(() => [props.dates, props.series], renderChart, { deep: true })
</script>
