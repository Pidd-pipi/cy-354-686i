import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listPriceAlerts,
  readPriceAlert as apiReadPriceAlert,
  deletePriceAlert as apiDeletePriceAlert,
} from '../api/favorite'
import type { PriceAlert } from '../types'

export const usePriceAlertStore = defineStore('priceAlert', () => {
  const alerts = ref<PriceAlert[]>([])
  const loaded = ref(false)

  const unreadCount = ref(0)

  async function fetchAlerts(force = false) {
    if (loaded.value && !force) return
    const res = await listPriceAlerts()
    alerts.value = res.data
    unreadCount.value = alerts.value.filter((a) => !a.is_read).length
    loaded.value = true
  }

  async function refreshUnreadCount() {
    const res = await listPriceAlerts()
    alerts.value = res.data
    unreadCount.value = alerts.value.filter((a) => !a.is_read).length
    loaded.value = true
  }

  async function markRead(id: number) {
    await apiReadPriceAlert(id)
    const a = alerts.value.find((x) => x.id === id)
    if (a && !a.is_read) {
      a.is_read = true
      unreadCount.value = Math.max(0, unreadCount.value - 1)
    }
  }

  async function remove(id: number) {
    await apiDeletePriceAlert(id)
    const a = alerts.value.find((x) => x.id === id)
    alerts.value = alerts.value.filter((x) => x.id !== id)
    if (a && !a.is_read) {
      unreadCount.value = Math.max(0, unreadCount.value - 1)
    }
  }

  function reset() {
    alerts.value = []
    unreadCount.value = 0
    loaded.value = false
  }

  return { alerts, loaded, unreadCount, fetchAlerts, refreshUnreadCount, markRead, remove, reset }
})
