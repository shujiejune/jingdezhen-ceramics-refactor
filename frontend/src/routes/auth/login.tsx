import { createFileRoute, Link, useNavigate } from "@tanstack/solid-router";
import { createSignal, Show } from "solid-js";
import { FormField, SocialLogins, MessageCard, AuthLayout } from "./_layout";

export const Route = createFileRoute("/auth/login")({
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const [email, setEmail] = createSignal("");
  const [password, setPassword] = createSignal("");
  const [isLoading, setIsLoading] = createSignal(false);
  const [serverError, setServerError] = createSignal<string | null>(null);

  const handleLogin = async (e: Event) => {
    e.preventDefault();
    setIsLoading(true);
    setServerError(null);
    // --- In a real app, you would make a fetch call here ---
    await new Promise((r) => setTimeout(r, 1000));
    // --- Simulate API response ---
    if (email() === "test@example.com" && password() === "password") {
      navigate({ to: "/" }); // Redirect on success
    } else {
      setServerError("Invalid email or password. Please try again.");
    }
    setIsLoading(false);
  };

  return (
    <>
      <div class="text-center">
        <h3 class="text-xl font-semibold">Log in to your account</h3>
      </div>

      <form class="space-y-6" onSubmit={handleLogin}>
        <Show when={serverError()}>
          <MessageCard
            type="error"
            title="Login Failed"
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

        <div class="text-sm text-right">
          <Link
            to="/auth/forgot-password"
            class="font-medium text-blue-600 hover:text-blue-500"
          >
            Forgot your password?
          </Link>
        </div>

        <div>
          <button
            type="submit"
            class="w-full btn-primary"
            disabled={isLoading()}
          >
            {isLoading() ? "Signing in..." : "Sign in"}
          </button>
        </div>
      </form>

      <SocialLogins verb="Sign in" />

      <p class="mt-6 text-center text-sm text-gray-600">
        Not a member?{" "}
        <Link
          to="/auth/signup"
          class="font-medium text-blue-600 hover:text-blue-500"
        >
          Sign up now
        </Link>
      </p>
    </>
  );
}
