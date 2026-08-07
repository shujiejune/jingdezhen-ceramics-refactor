import { createFileRoute, Link, useSearch } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { Show } from "solid-js";
import { z } from "zod";
import { MessageCard } from "./_layout";

const activateSearchSchema = z.object({
  token: z.string().optional(),
});

// Mock API call to verify the token
const activateAccount = async (token: string | undefined) => {
  console.log("Activating with token:", token);
  await new Promise((r) => setTimeout(r, 1500));
  if (token && token.length > 10) {
    return {
      success: true,
      message: "Your account has been successfully activated!",
    };
  }
  return {
    success: false,
    message: "This activation link is invalid or has expired.",
  };
};

export const Route = createFileRoute("/auth/activate")({
  validateSearch: (search) => activateSearchSchema.parse(search),
  component: AccountActivationPage,
});

function AccountActivationPage() {
  const search = Route.useSearch();
  const activationQuery = useQuery(() => ({
    queryKey: ["activation", search().token],
    queryFn: () => activateAccount(search().token),
  }));

  return (
    <div class="text-center">
      <Show when={activationQuery.isLoading}>
        <h3 class="text-xl font-semibold">Activating your account...</h3>
        <p class="mt-2 text-gray-600">Please wait a moment.</p>
      </Show>
      <Show when={activationQuery.isError}>
        <MessageCard
          type="error"
          title="Activation Failed"
          message="An unexpected error occurred. Please try again later."
        />
      </Show>
      <Show when={activationQuery.data}>
        {(data) => (
          <>
            <MessageCard
              type={data().success ? "success" : "error"}
              title={
                data().success ? "Activation Successful!" : "Activation Failed"
              }
              message={data().message}
            />
            <div class="mt-6">
              <Link
                to="/auth/login"
                class="w-full btn-primary inline-flex justify-center"
              >
                Proceed to Login
              </Link>
            </div>
          </>
        )}
      </Show>
    </div>
  );
}
