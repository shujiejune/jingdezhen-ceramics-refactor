import { createFileRoute, Link, useSearch } from "@tanstack/solid-router";
import { createSignal, Show } from "solid-js";
import { z } from "zod";
import { FormField, MessageCard, AuthLayout } from "./_layout";

const resetSearchSchema = z.object({
  token: z.string().optional(),
});

export const Route = createFileRoute("/auth/reset-password")({
  validateSearch: (search) => resetSearchSchema.parse(search),
  component: ResetPasswordPage,
});

function ResetPasswordPage() {
  const search = Route.useSearch();
  const [password, setPassword] = createSignal("");
  const [confirmPassword, setConfirmPassword] = createSignal("");
  const [isLoading, setIsLoading] = createSignal(false);
  const [serverError, setServerError] = createSignal<string | null>(null);
  const [success, setSuccess] = createSignal(false);

  const handleReset = async (e: Event) => {
    e.preventDefault();
    if (!search().token) {
      setServerError("Invalid or missing reset token.");
      return;
    }
    if (password() !== confirmPassword()) {
      setServerError("Passwords do not match.");
      return;
    }
    setIsLoading(true);
    setServerError(null);
    // Simulate API call
    await new Promise((r) => setTimeout(r, 1000));
    setSuccess(true);
  };

  return (
    <Show
      when={success()}
      fallback={
        <>
          <div class="text-center mb-6">
            <h3 class="text-xl font-semibold">Set a new password</h3>
          </div>
          <Show when={!search().token}>
            <MessageCard
              type="error"
              title="Error"
              message="This password reset link is invalid or has expired."
            />
          </Show>
          <form class="space-y-4" onSubmit={handleReset}>
            <Show when={serverError()}>
              <MessageCard
                type="error"
                title="Error"
                message={serverError()!}
              />
            </Show>
            <FormField
              label="New Password"
              name="password"
              type="password"
              value={password()}
              onInput={(e) => setPassword(e.currentTarget.value)}
            />
            <FormField
              label="Confirm New Password"
              name="confirm-password"
              type="password"
              value={confirmPassword()}
              onInput={(e) => setConfirmPassword(e.currentTarget.value)}
            />
            <div>
              <button
                type="submit"
                class="w-full btn-primary"
                disabled={isLoading() || !search().token}
              >
                {isLoading() ? "Resetting..." : "Reset Password"}
              </button>
            </div>
          </form>
        </>
      }
    >
      <MessageCard
        type="success"
        title="Password Reset!"
        message="Your password has been successfully updated. You can now log in with your new password."
      />
      <div class="mt-4 text-center">
        <Link
          to="/auth/login"
          class="font-medium text-blue-600 hover:text-blue-500"
        >
          Proceed to Login
        </Link>
      </div>
    </Show>
  );
}
