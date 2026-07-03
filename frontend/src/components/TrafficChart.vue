<template>
  <div class="traffic-chart" v-show="!collapsed">
    <div class="traffic-header">
      <span class="traffic-title">VPN 流量</span>
      <span class="traffic-rate">
        <span class="up">↑ {{ formatSpeed(upRate) }}</span>
        <span class="down">↓ {{ formatSpeed(downRate) }}</span>
      </span>
    </div>
    <canvas ref="canvas" :width="width" :height="height"></canvas>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps({
  collapsed: Boolean,
  data: {
    type: Array,
    default: () => []
  }
})

const canvas = ref(null)
const width = 184
const height = 60
const upRate = ref(0)
const downRate = ref(0)

const points = []
const maxPoints = 60

const formatSpeed = (bytesPerSec) => {
  if (bytesPerSec === 0) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let i = 0
  let value = bytesPerSec
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return value.toFixed(1) + ' ' + units[i]
}

const draw = () => {
  const ctx = canvas.value?.getContext('2d')
  if (!ctx) return

  ctx.clearRect(0, 0, width, height)

  if (points.length < 2) return

  let max = 0
  for (const p of points) {
    if (p.up > max) max = p.up
    if (p.down > max) max = p.down
  }
  if (max === 0) return

  const padding = 4
  const chartHeight = height - padding * 2

  const drawLine = (key, color) => {
    ctx.beginPath()
    ctx.strokeStyle = color
    ctx.lineWidth = 1.5
    for (let i = 0; i < points.length; i++) {
      const x = (i / (maxPoints - 1)) * width
      const y = height - padding - (points[i][key] / max) * chartHeight
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    }
    ctx.stroke()
  }

  drawLine('down', '#3b82f6')
  drawLine('up', '#10b981')
}

const pushData = (up, down) => {
  points.push({ up, down })
  if (points.length > maxPoints) points.shift()
  draw()
}

watch(() => props.data, (newData) => {
  if (!newData || newData.length === 0) return
  const latest = newData[newData.length - 1]
  upRate.value = latest.up || 0
  downRate.value = latest.down || 0
  pushData(upRate.value, downRate.value)
}, { deep: true })

onMounted(() => {
  draw()
})
</script>

<style scoped>
.traffic-chart {
  margin-top: auto;
  padding: 12px;
  border-top: 1px solid #333;
}

.traffic-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 12px;
}

.traffic-title {
  color: #aaa;
}

.traffic-rate {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.traffic-rate .up {
  color: #10b981;
}

.traffic-rate .down {
  color: #3b82f6;
}

canvas {
  width: 100%;
  height: 60px;
  background: #0f1a30;
  border-radius: 6px;
}
</style>
