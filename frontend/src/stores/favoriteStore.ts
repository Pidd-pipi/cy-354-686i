import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  addFavorite as apiAddFavorite,
  removeFavorite as apiRemoveFavorite,
  listMyFavoriteIds,
} from '../api/favorite'

export const useFavoriteStore = defineStore('favorite', () => {
  const favoritedIds = ref<Set<number>>(new Set())
  const loaded = ref(false)

  function has(productId: number): boolean {
    return favoritedIds.value.has(productId)
  }

  async function hydrate() {
    if (loaded.value) return
    const res = await listMyFavoriteIds()
    favoritedIds.value = new Set(res.data.product_ids)
    loaded.value = true
  }

  function reset() {
    favoritedIds.value = new Set()
    loaded.value = false
  }

  async function toggle(productId: number) {
    const isFav = favoritedIds.value.has(productId)
    // Optimistic update; roll back on failure.
    if (isFav) {
      favoritedIds.value.delete(productId)
      favoritedIds.value = new Set(favoritedIds.value)
      try {
        await apiRemoveFavorite(productId)
      } catch (e) {
        favoritedIds.value.add(productId)
        favoritedIds.value = new Set(favoritedIds.value)
        throw e
      }
    } else {
      favoritedIds.value.add(productId)
      favoritedIds.value = new Set(favoritedIds.value)
      try {
        await apiAddFavorite(productId)
      } catch (e) {
        favoritedIds.value.delete(productId)
        favoritedIds.value = new Set(favoritedIds.value)
        throw e
      }
    }
  }

  // applyDelta adjusts the favorite count shown on a card after a toggle.
  function applyDelta(products: { id: number; favorite_count: number }[], productId: number, delta: number) {
    const p = products.find((x) => x.id === productId)
    if (p) {
      p.favorite_count = Math.max(0, p.favorite_count + delta)
    }
  }

  return { favoritedIds, loaded, has, hydrate, reset, toggle, applyDelta }
})
