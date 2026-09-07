<script setup lang="ts">
// A dashboard box. Registers itself with the grid on mount so the user can
// drag/resize it in edit mode; its position is remembered per screen.
import { inject, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { DASH_KEY } from '../dashboard'

const props = defineProps<{ id: string; w?: number; h?: number }>()
const dash = inject(DASH_KEY)
const el = ref<HTMLElement | null>(null)

onMounted(async () => {
  await nextTick()
  if (el.value) dash?.add(props.id, el.value, { w: props.w, h: props.h })
})
onBeforeUnmount(() => {
  if (el.value) dash?.remove(el.value)
})
</script>

<template>
  <div ref="el" class="grid-stack-item" :gs-id="id">
    <section class="panel grid-stack-item-content">
      <div class="wcontent"><slot /></div>
    </section>
  </div>
</template>
