<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "@/api/client";
import type { components } from "@/api/schema";

type AdminStats = components["schemas"]["AdminStats"];

const router = useRouter();
const stats = ref<AdminStats | null>(null);
const error = ref("");

async function load() {
  error.value = "";

  const { data, response } = await api.GET("/api/admin/stats");

  if (response.status === 401) {
    await router.replace("/sign-in");
    return;
  }
  if (response.status === 403) {
    error.value = "You do not have access to this page.";
    return;
  }
  if (!data) {
    error.value = "Unable to load statistics.";
    return;
  }
  stats.value = data;
}

onMounted(load);
</script>

<template>
  <UCard>
    <template #header>
      <h1 class="text-2xl font-semibold">Admin</h1>
    </template>

    <UAlert v-if="error" color="error" :title="error" />

    <div
      v-else-if="stats"
      class="divide-y divide-gray-200 dark:divide-gray-800"
    >
      <div class="flex items-center justify-between gap-4 py-3">
        <span class="text-sm text-gray-500 dark:text-gray-400">
          User accounts
        </span>
        <span class="font-semibold tabular-nums">
          {{ stats.user_accounts }}
        </span>
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <span class="text-sm text-gray-500 dark:text-gray-400">
          Active users
        </span>
        <span class="font-semibold tabular-nums">
          {{ stats.active_users }}
        </span>
      </div>

      <div class="flex items-center justify-between gap-4 py-3">
        <span class="text-sm text-gray-500 dark:text-gray-400">
          Remaining images
        </span>
        <span class="font-semibold tabular-nums">
          {{ stats.remaining_files }}
        </span>
      </div>
    </div>
  </UCard>
</template>
