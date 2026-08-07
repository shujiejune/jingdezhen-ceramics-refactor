import { createFileRoute, Link } from "@tanstack/solid-router";
import { createSignal, Show } from "solid-js";
import { FormField, MessageCard, AuthLayout } from "./_layout";

export const Route = createFileRoute("/auth/forgot-password")({
  component: ForgotPasswordPage,
});

function ForgotPasswordPage() {
  const [email, setEmail] = createSignal("");
  const [isLoading, setIsLoading] = createSignal(false);
  const [formSubmitted, setFormSubmitted] = createSignal(false);

  const handleRequest = async (e: Event) => {
    e.preventDefault();
    setIsLoading(true);
    // Simulate API call
    await new Promise((r) => setTimeout(r, 1000));
    setFormSubmitted(true);
  };

  return (
    <Show
      when={formSubmitted()}
      fallback={
        <>
          <div class="text-center mb-6">
            <h3 class="text-xl font-semibold">Reset your password</h3>
            <p class="text-sm text-gray-500 mt-2">
              Enter your email and we'll send you a link to get back into your
              account.
            </p>
          </div>
          <form class="space-y-6" onSubmit={handleRequest}>
            <FormField
              label="Email address"
              name="email"
              type="email"
              value={email()}
              onInput={(e) => setEmail(e.currentTarget.value)}
            />
            <div>
              <button
                type="submit"
                class="w-full btn-primary"
                disabled={isLoading()}
              >
                {isLoading() ? "Sending..." : "Send Reset Link"}
              </button>
            </div>
          </form>
        </>
      }
    >
      <MessageCard
        type="success"
        title="Check Your Email"
        message={`If an account exists for ${email()}, you will receive an email with instructions on how to reset your password.`}
      />
      <div class="mt-4 text-center">
        <Link
          to="/auth/login"
          class="font-medium text-blue-600 hover:text-blue-500"
        >
          &larr; Back to Login
        </Link>
      </div>
    </Show>
  );
}
