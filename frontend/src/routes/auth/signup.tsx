import { createFileRoute, Link } from "@tanstack/solid-router";
import { createSignal, Show } from "solid-js";
import { FormField, SocialLogins, MessageCard, AuthLayout } from "./_layout";

export const Route = createFileRoute("/auth/signup")({
  component: SignupPage,
});

function SignupPage() {
  const [email, setEmail] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [confirmPassword, setConfirmPassword] = createSignal("");
  const [isLoading, setIsLoading] = createSignal(false);
  const [serverError, setServerError] = createSignal<string | null>(null);
  const [formSubmitted, setFormSubmitted] = createSignal(false);

  const handleSignup = async (e: Event) => {
    e.preventDefault();
    if (password() !== confirmPassword()) {
      setServerError("Passwords do not match.");
      return;
    }
    setIsLoading(true);
    setServerError(null);

    // --- In a real app, you would make a fetch call here ---
    await new Promise((r) => setTimeout(r, 1000));

    // --- Simulate API response ---
    if (email() === "exists@example.com") {
      setServerError("An account with this email already exists.");
      setIsLoading(false);
    } else {
      setFormSubmitted(true); // Show success message
    }
  };

  return (
    <Show
      when={formSubmitted()}
      fallback={
        <>
          <div class="text-center">
            <h3 class="text-xl font-semibold">Create a new account</h3>
          </div>
          <form class="space-y-4" onSubmit={handleSignup}>
            <Show when={serverError()}>
              <MessageCard
                type="error"
                title="Registration Failed"
                message={serverError()!}
              />
            </Show>
            <FormField
              label="Email address"
              name="email"
              type="email"
              value={email()}
              onInput={(e) => setEmail(e.currentTarget.value)}
            />
            <FormField
              label="Password"
              name="password"
              type="password"
              value={password()}
              onInput={(e) => setPassword(e.currentTarget.value)}
            />
            <FormField
              label="Confirm Password"
              name="confirm-password"
              type="password"
              value={confirmPassword()}
              onInput={(e) => setConfirmPassword(e.currentTarget.value)}
            />
            <div>
              <button
                type="submit"
                class="w-full btn-primary"
                disabled={isLoading()}
              >
                {isLoading() ? "Creating account..." : "Create Account"}
              </button>
            </div>
          </form>

          <SocialLogins verb="Sign up" />

          <p class="mt-6 text-center text-sm text-gray-600">
            Already have an account?{" "}
            <Link
              to="/auth/login"
              class="font-medium text-blue-600 hover:text-blue-500"
            >
              Log in
            </Link>
          </p>
        </>
      }
    >
      <MessageCard
        type="success"
        title="Account Created!"
        message="We've sent an activation link to your email address. Please check your inbox and click the link to activate your account."
      />
      <div class="mt-4 text-center">
        <Link
          to="/auth/login"
          class="font-medium text-blue-600 hover:text-blue-500"
        >
          Back to Login
        </Link>
      </div>
    </Show>
  );
}
