<template>
  <Select
    :model-value="modelValue"
    :options="displayOptions as unknown as Array<Record<string, unknown>>"
    :placeholder="t('keys.selectGroup')"
    :empty-text="t('common.noGroupsAvailable')"
    :searchable="true"
    :search-placeholder="t('keys.searchGroup')"
    @update:model-value="onChange"
  >
    <template #selected="{ option }">
      <span v-if="option && (option as unknown as DisplayOption).unavailable" class="text-amber-600">
        {{ (option as unknown as DisplayOption).label }}
      </span>
      <GroupBadge
        v-else-if="option"
        :name="(option as unknown as DisplayOption).label"
        :platform="(option as unknown as DisplayOption).platform"
        :subscription-type="(option as unknown as DisplayOption).subscriptionType"
        :rate-multiplier="(option as unknown as DisplayOption).rate"
        :user-rate-multiplier="(option as unknown as DisplayOption).userRate"
        :peak-rate-enabled="(option as unknown as DisplayOption).peakRateEnabled"
        :peak-start="(option as unknown as DisplayOption).peakStart"
        :peak-end="(option as unknown as DisplayOption).peakEnd"
        :peak-rate-multiplier="(option as unknown as DisplayOption).peakRateMultiplier"
      />
      <span v-else class="text-gray-400">{{ t('keys.selectGroup') }}</span>
    </template>
    <template #option="{ option, selected }">
      <span v-if="(option as unknown as DisplayOption).unavailable" class="text-amber-600">
        {{ (option as unknown as DisplayOption).label }}
      </span>
      <GroupOptionItem
        v-else
        :name="(option as unknown as DisplayOption).label"
        :platform="(option as unknown as DisplayOption).platform"
        :subscription-type="(option as unknown as DisplayOption).subscriptionType"
        :rate-multiplier="(option as unknown as DisplayOption).rate"
        :user-rate-multiplier="(option as unknown as DisplayOption).userRate"
        :peak-rate-enabled="(option as unknown as DisplayOption).peakRateEnabled"
        :peak-start="(option as unknown as DisplayOption).peakStart"
        :peak-end="(option as unknown as DisplayOption).peakEnd"
        :peak-rate-multiplier="(option as unknown as DisplayOption).peakRateMultiplier"
        :description="(option as unknown as DisplayOption).description"
        :selected="selected"
      />
    </template>
  </Select>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type { SubscriptionType, GroupPlatform } from '@/types'

interface GroupOption {
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
}

type DisplayOption = GroupOption & { disabled?: boolean; unavailable?: boolean }

const props = defineProps<{
  modelValue: number | null
  options: GroupOption[]
  unavailableId?: number
}>()
const emit = defineEmits<{ (e: 'update:modelValue', value: number | null): void }>()
const { t } = useI18n()

const displayOptions = computed<DisplayOption[]>(() => {
  if (!props.unavailableId || props.options.some((group) => group.value === props.unavailableId)) {
    return props.options
  }
  return [{
    value: props.unavailableId,
    label: t('keys.groupBindings.unavailable', { id: props.unavailableId }),
    description: null,
    rate: 1,
    userRate: null,
    peakRateEnabled: false,
    peakStart: '',
    peakEnd: '',
    peakRateMultiplier: 1,
    subscriptionType: 'standard',
    platform: 'openai',
    disabled: true,
    unavailable: true
  }, ...props.options]
})

const onChange = (value: string | number | boolean | null) => {
  emit('update:modelValue', typeof value === 'number' ? value : null)
}
</script>
