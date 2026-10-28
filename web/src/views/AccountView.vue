<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "@/api/client";
import type { components } from "@/api/schema";

type Account = components["schemas"]["Account"];

const router = useRouter();
const account = ref<Account | null>(null);
const error = ref("");
const toggling = ref(false);

const nextCloudsTime = computed(() => {
  const at = account.value?.next_clouds_at;
  if (!at) return "";
  const minutes = Math.max(
    0,
    Math.round((new Date(at).getTime() - Date.now()) / 60000),
  );
  const hours = Math.floor(minutes / 60);
  return hours > 0 ? `${hours}h ${minutes % 60}m` : `${minutes}m`;
});

async function load() {
  error.value = "";

  const { data, response } = await api.GET("/api/account");

  if (response.status === 401) {
    await router.replace("/sign-in");
    return;
  }
  if (!data) {
    error.value = "Unable to load your account.";
    return;
  }
  account.value = data;
}

async function setActive(active: boolean) {
  if (toggling.value) return;
  toggling.value = true;
  error.value = "";

  try {
    const { data, response } = await api.POST("/api/account/active", {
      body: { active },
    });

    if (response.status === 401) {
      await router.replace("/sign-in");
      return;
    }
    if (!data) {
      error.value = "Unable to update your account.";
      return;
    }
    if (account.value) account.value.active = data.active;
  } finally {
    toggling.value = false;
  }
}

async function logout(all: boolean) {
  await (all ? api.POST("/auth/logout-all") : api.POST("/auth/logout"));
  await router.replace("/sign-in");
}

async function deleteAccount() {
  if (!confirm("Delete your account permanently? This cannot be undone."))
    return;
  await api.DELETE("/api/account");
  await router.replace("/sign-in");
}

onMounted(load);
</script>

<template>
  <UCard>
    <template #header>
      <div class="flex items-center justify-between gap-4">
        <h1 class="text-2xl font-semibold">Your account</h1>
        <UBadge v-if="account" :color="account.active ? 'success' : 'neutral'">
          {{ account.active ? "Active" : "Paused" }}
        </UBadge>
      </div>
      <p
        v-if="account"
        class="mt-1 break-all text-sm text-gray-500 dark:text-gray-400"
      >
        {{ account.email }}
      </p>
    </template>

    <UAlert v-if="error" color="error" :title="error" />

    <template v-else-if="account">
      <div class="flex items-center justify-between gap-4">
        <div>
          <h2 class="text-lg font-medium">Daily emails</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{
              account.active
                ? "Emails are being delivered to your inbox."
                : "Delivery is currently paused."
            }}
          </p>
          <p
            v-if="account.active && nextCloudsTime"
            class="mt-1 text-sm text-gray-500 dark:text-gray-400"
          >
            Next clouds in
            <span class="font-medium tabular-nums text-gray-900 dark:text-white">
              {{ nextCloudsTime }}
            </span>
          </p>
        </div>
        <UButton
          color="neutral"
          variant="soft"
          :loading="toggling"
          @click="setActive(!account.active)"
        >
          {{ account.active ? "Pause emails" : "Resume emails" }}
        </UButton>
      </div>

      <USeparator class="my-6" />

      <div class="space-y-3">
        <UButton
          block
          color="neutral"
          variant="soft"
          @click="logout(false)"
        >
          Sign out
        </UButton>
        <UButton
          block
          color="neutral"
          variant="soft"
          @click="logout(true)"
        >
          Sign out everywhere
        </UButton>
        <UButton
          block
          color="error"
          variant="soft"
          @click="deleteAccount"
        >
          Delete account
        </UButton>
      </div>
    </template>
  </UCard>
</template>
