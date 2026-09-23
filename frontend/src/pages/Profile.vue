<template>
  <div class="page">
    <h2>个人中心</h2>
    <template v-if="authStore.user">
      <el-card class="profile-card">
        <div class="profile-head">
          <el-avatar :size="64" :src="authStore.user.avatar || ''">{{ authStore.user.nickname.slice(0, 1) }}</el-avatar>
          <div class="profile-info">
            <h3>{{ authStore.user.nickname }}</h3>
            <p>{{ authStore.user.phone }} · {{ roleLabel(authStore.user.role) }} · {{ authStore.user.campus }}</p>
          </div>
          <div class="credit-box">
            <div class="credit-label">信誉分</div>
            <div class="credit-value">{{ authStore.user.credit_score }}</div>
            <div class="credit-level">{{ creditLevel(authStore.user.credit_score) }}</div>
          </div>
        </div>
      </el-card>

      <!-- 降价提醒 -->
      <el-card class="section">
        <template #header>
          <div class="section-head">
            <span>🔔 降价提醒 <el-badge v-if="unreadAlerts > 0" :value="unreadAlerts" class="alert-badge" /></span>
            <el-button v-if="unreadAlerts > 0" link type="primary" @click="readAll">全部标为已读</el-button>
          </div>
        </template>
        <el-empty v-if="alerts.length === 0" description="暂无降价提醒" :image-size="60" />
        <div v-for="a in alerts" :key="a.id" class="alert-item" :class="{ unread: !a.read }">
          <div class="alert-main">
            <div class="alert-title">
              <el-tag size="small" type="danger">降价</el-tag>
              {{ a.product?.title ?? `商品 #${a.product_id}` }}
              <span class="alert-price">¥{{ a.old_price.toFixed(2) }} → <b>¥{{ a.new_price.toFixed(2) }}</b></span>
            </div>
            <div class="alert-meta">
              {{ formatDateTime(a.created_at) }}
              <el-tag v-if="a.product" size="small" :type="productStatusType(a.product.status) as any">
                {{ productStatusLabel(a.product.status) }}
              </el-tag>
            </div>
          </div>
          <div class="alert-actions">
            <el-button
              v-if="a.product?.purchasable"
              size="small"
              type="primary"
              @click="buyFromAlert(a)"
            >去购买</el-button>
            <el-button size="small" @click="chatFromAlert(a)">私信</el-button>
            <el-button v-if="!a.read" size="small" link @click="readOne(a.id)">标为已读</el-button>
          </div>
        </div>
      </el-card>

      <!-- 我的收藏 -->
      <el-card class="section">
        <template #header>⭐ 我的收藏（{{ favorites.length }}）</template>
        <el-empty v-if="favorites.length === 0" description="还没有收藏商品，去商品广场看看吧" :image-size="60" />
        <el-row :gutter="16">
          <el-col v-for="f in favorites" :key="f.product_id" :span="8" class="fav-col">
            <el-card shadow="never" class="fav-card" :class="{ offsale: !f.purchasable }">
              <div class="fav-head">
                <h4>{{ f.title }}</h4>
                <el-tag size="small" :type="productStatusType(f.status) as any">{{ productStatusLabel(f.status) }}</el-tag>
              </div>
              <div class="fav-price">¥{{ f.price.toFixed(2) }} <span class="fav-count">⭐ {{ f.favorite_count }}人收藏</span></div>
              <div class="fav-meta">{{ categoryLabel(f.category) }} · {{ f.campus }} · {{ f.condition }}</div>
              <div class="fav-actions">
                <el-button size="small" type="primary" :disabled="!f.purchasable" @click="buyFav(f)">
                  {{ f.purchasable ? '购买' : '不可购买' }}
                </el-button>
                <el-button size="small" :disabled="!f.purchasable" @click="chatFav(f)">私信</el-button>
                <el-button size="small" type="danger" plain @click="unfavorite(f)">取消收藏</el-button>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-card>

      <!-- 我发布的商品 -->
      <el-card class="section">
        <template #header>🏷️ 我发布的商品（{{ myProducts.length }}）</template>
        <el-empty v-if="myProducts.length === 0" description="还没有发布商品" :image-size="60" />
        <el-table v-else :data="myProducts">
          <el-table-column prop="title" label="商品" min-width="160" />
          <el-table-column label="价格" width="180">
            <template #default="{ row }">
              <span :class="{ 'price-drop': editingId === row.id }">¥{{ row.price.toFixed(2) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="收藏人数" width="100">
            <template #default="{ row }">⭐ {{ row.favorite_count ?? 0 }}</template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="productStatusType(row.status) as any">{{ productStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" min-width="240">
            <template #default="{ row }">
              <template v-if="row.status === 'on_sale'">
                <el-button v-if="editingId !== row.id" size="small" @click="startEdit(row)">改价</el-button>
                <template v-else>
                  <el-input-number v-model="editPrice" :min="0.01" :precision="2" :step="1" size="small" style="width: 120px" />
                  <el-button size="small" type="primary" @click="savePrice(row)">保存</el-button>
                  <el-button size="small" @click="editingId = null">取消</el-button>
                </template>
                <el-button size="small" type="danger" plain @click="takeDown(row)">下架</el-button>
              </template>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- 收到的评价 -->
      <el-card class="section">
        <template #header>📝 收到的评价</template>
        <el-table :data="reviews">
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column label="评价">
            <template #default="{ row }">
              <el-tag size="small">{{ ratingLabel(row.rating) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="content" label="内容" />
          <el-table-column label="时间">
            <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </el-card>
    </template>
    <el-empty v-else description="请先登录">
      <el-button type="primary" @click="$router.push('/login')">去登录</el-button>
    </el-empty>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/authStore'
import { roleLabel } from '../constants/user'
import { ratingLabel } from '../constants/trade'
import { categoryLabel, productStatusLabel, productStatusType } from '../constants/product'
import { listMyReviews } from '../api/review'
import {
  listMyFavorites, listMyPriceAlerts, markAllPriceAlertsRead, markPriceAlertRead,
  removeFavorite, listMyProducts, updateProductPrice,
} from '../api/favorite'
import { removeProduct } from '../api/product'
import { createTradeOrder } from '../api/tradeOrder'
import { createConversation } from '../api/conversation'
import { formatDateTime } from '../utils/dateFormat'
import type { FavoriteProduct, PriceAlert, Product, Review } from '../types'
import { useRouter } from 'vue-router'

const authStore = useAuthStore()
const router = useRouter()
const reviews = ref<Review[]>([])
const favorites = ref<FavoriteProduct[]>([])
const alerts = ref<PriceAlert[]>([])
const myProducts = ref<Product[]>([])
const editingId = ref<number | null>(null)
const editPrice = ref(0)

const unreadAlerts = computed(() => alerts.value.filter((a) => !a.read).length)

function creditLevel(score: number): string {
  if (score >= 200) return '极佳'
  if (score >= 150) return '优秀'
  if (score >= 100) return '良好'
  if (score >= 60) return '一般'
  return '待提升'
}

async function loadAll() {
  if (!authStore.token) return
  const [favRes, alertRes, prodRes] = await Promise.all([
    listMyFavorites(), listMyPriceAlerts(), listMyProducts(),
  ])
  favorites.value = favRes.data.items
  alerts.value = alertRes.data.items
  myProducts.value = prodRes.data.items
  const reviewRes = await listMyReviews()
  reviews.value = reviewRes.data
}

async function unfavorite(f: FavoriteProduct) {
  await removeFavorite(f.product_id)
  favorites.value = favorites.value.filter((x) => x.product_id !== f.product_id)
  ElMessage.success('已取消收藏')
}

async function buyFav(f: FavoriteProduct) {
  await createTradeOrder(f.product_id)
  ElMessage.success('已下单，等待卖家确认')
}

async function chatFav(f: FavoriteProduct) {
  await createConversation(f.product_id)
  ElMessage.success('已发起私信')
  router.push('/messages')
}

async function buyFromAlert(a: PriceAlert) {
  await createTradeOrder(a.product_id)
  ElMessage.success('已下单，等待卖家确认')
}

async function chatFromAlert(a: PriceAlert) {
  await createConversation(a.product_id)
  ElMessage.success('已发起私信')
  router.push('/messages')
}

async function readOne(id: number) {
  await markPriceAlertRead(id)
  const target = alerts.value.find((a) => a.id === id)
  if (target) target.read = true
}

async function readAll() {
  await markAllPriceAlertsRead()
  alerts.value.forEach((a) => (a.read = true))
}

function startEdit(row: Product) {
  editingId.value = row.id
  editPrice.value = row.price
}

async function savePrice(row: Product) {
  if (!(editPrice.value > 0)) {
    ElMessage.warning('价格必须大于 0')
    return
  }
  if (editPrice.value === row.price) {
    editingId.value = null
    return
  }
  const dropped = editPrice.value < row.price
  await updateProductPrice(row.id, editPrice.value)
  row.price = editPrice.value
  editingId.value = null
  ElMessage.success(dropped ? '已降价，收藏该商品的学生会收到提醒' : '价格已更新')
}

async function takeDown(row: Product) {
  try {
    await ElMessageBox.confirm(`确定下架「${row.title}」吗？收藏仍会保留在学生的个人中心。`, '下架商品', {
      type: 'warning',
    })
  } catch {
    return
  }
  await removeProduct(row.id)
  row.status = 'removed'
  // 收藏行保留，但状态变为不可购买
  favorites.value.forEach((f) => {
    if (f.product_id === row.id) {
      f.status = 'removed'
      f.purchasable = false
    }
  })
  ElMessage.success('已下架')
}

onMounted(loadAll)
</script>

<style scoped>
.profile-card {
  margin-bottom: 16px;
}
.profile-head {
  display: flex;
  align-items: center;
  gap: 16px;
}
.profile-info h3 {
  margin: 0;
}
.profile-info p {
  margin: 4px 0 0;
  color: #909399;
  font-size: 13px;
}
.credit-box {
  margin-left: auto;
  text-align: center;
}
.credit-label {
  color: #909399;
  font-size: 12px;
}
.credit-value {
  font-size: 28px;
  font-weight: 700;
  color: #e6a23c;
}
.credit-level {
  font-size: 12px;
  color: #909399;
}
.section {
  margin-bottom: 16px;
}
.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.alert-badge {
  margin-left: 4px;
}
.alert-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
}
.alert-item.unread {
  background: #fdf6ec;
  border-radius: 4px;
}
.alert-title {
  font-size: 14px;
  color: #303133;
  display: flex;
  gap: 8px;
  align-items: center;
}
.alert-price {
  color: #f56c6c;
}
.alert-price b {
  font-size: 16px;
}
.alert-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
  display: flex;
  gap: 8px;
  align-items: center;
}
.fav-col {
  margin-bottom: 16px;
}
.fav-card.offsale {
  background: #fafafa;
  opacity: 0.85;
}
.fav-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.fav-head h4 {
  margin: 0;
  font-size: 15px;
}
.fav-price {
  color: #f56c6c;
  font-size: 18px;
  font-weight: 700;
  margin: 8px 0;
}
.fav-count {
  font-size: 12px;
  font-weight: 400;
  color: #e6a23c;
}
.fav-meta {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}
.fav-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.muted {
  color: #c0c4cc;
}
</style>
