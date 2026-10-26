<script setup lang="ts">
import { ref } from "vue";
import { api } from "@/api/client";

const email = ref("");
const loading = ref(false);
const message = ref("");
const error = ref("");

async function submit() {
  loading.value = true;
  message.value = "";
  error.value = "";

  const { error: requestError } = await api.POST("/auth/magic-link", {
    body: { email: email.value },
  });

  if (requestError) {
    error.value = "Something went wrong. Try again.";
  } else {
    message.value = "A sign-in link has been sent.";
  }
  loading.value = false;
}
</script>

<template>
  <UCard>
    <template #header>
      <h1 class="text-2xl font-semibold">Sign in</h1>
    </template>

    <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
      Enter your email address:
    </p>

    <form class="space-y-4" @submit.prevent="submit">
      <UInput
        v-model="email"
        type="email"
        placeholder="you@example.com"
        autocomplete="email"
        required
        class="w-full"
      />
      <UButton type="submit" :loading="loading" block>Get sign-in link</UButton>
    </form>

    <div class="reveal" :class="{ 'reveal-open': !!(message || error) }">
      <div class="reveal-body">
        <UAlert v-if="message" color="success" :title="message" class="mt-4" />
        <UAlert v-if="error" color="error" :title="error" class="mt-4" />
      </div>
    </div>
  </UCard>
</template>

<style scoped>
.reveal {
  display: grid;
  grid-template-rows: 0fr;
  opacity: 0;
  transition:
    grid-template-rows 200ms ease,
    opacity 200ms ease;
}

.reveal-open {
  grid-template-rows: 1fr;
  opacity: 1;
}

.reveal-body {
  min-height: 0;
  overflow: hidden;
}
</style>
