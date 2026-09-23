import { ref } from 'vue'
import { addFavorite, listMyFavoriteIds, removeFavorite } from '../api/favorite'
import { useAuthStore } from '../stores/authStore'

// Module-level singleton: favorited product ids shared across pages.
const favoriteIds = ref<Set<number>>(new Set())
const loaded = ref(false)
let inflight: Promise<void> | null = null

async function loadFavoriteIds(force = false): Promise<void> {
  const authStore = useAuthStore()
  if (!authStore.token) {
    favoriteIds.value = new Set()
    loaded.value = false
    return
  }
  if (loaded.value && !force) return
  if (inflight) return inflight
  inflight = (async () => {
    try {
      const res = await listMyFavoriteIds()
      favoriteIds.value = new Set(res.data.product_ids)
    } catch {
      favoriteIds.value = new Set()
    } finally {
      loaded.value = true
      inflight = null
    }
  })()
  return inflight
}

export function useFavorites() {
  function isFavorite(productId: number): boolean {
    return favoriteIds.value.has(productId)
  }

  async function toggleFavorite(productId: number): Promise<boolean> {
    const authStore = useAuthStore()
    if (!authStore.token) {
      return false
    }
    const next = new Set(favoriteIds.value)
    if (next.has(productId)) {
      await removeFavorite(productId)
      next.delete(productId)
    } else {
      await addFavorite(productId)
      next.add(productId)
    }
    favoriteIds.value = next
    return next.has(productId)
  }

  function reset() {
    favoriteIds.value = new Set()
    loaded.value = false
    inflight = null
  }

  return { favoriteIds, isFavorite, toggleFavorite, loadFavoriteIds, reset }
}
