import request from '../utils/request'
import type { FavoriteItem, PriceAlert } from '../types'

type Envelope<T> = { code: number; message: string; data: T }

export function addFavorite(productId: number) {
  return request.post<never, Envelope<{ product_id: number; favorited: boolean }>>(
    `/products/${productId}/favorites`,
  )
}

export function removeFavorite(productId: number) {
  return request.delete<never, Envelope<{ product_id: number; favorited: boolean }>>(
    `/products/${productId}/favorites`,
  )
}

export function listMyFavorites() {
  return request.get<never, Envelope<FavoriteItem[]>>('/me/favorites')
}

export function listMyFavoriteIds() {
  return request.get<never, Envelope<{ product_ids: number[] }>>('/me/favorite-ids')
}

export function listPriceAlerts() {
  return request.get<never, Envelope<PriceAlert[]>>('/me/price-alerts')
}

export function readPriceAlert(id: number) {
  return request.put<never, Envelope<{ id: number; is_read: boolean }>>(`/me/price-alerts/${id}/read`)
}

export function deletePriceAlert(id: number) {
  return request.delete<never, Envelope<{ id: number }>>(`/me/price-alerts/${id}`)
}
