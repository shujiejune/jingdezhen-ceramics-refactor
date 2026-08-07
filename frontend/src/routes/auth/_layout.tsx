import { createFileRoute, Outlet } from "@tanstack/solid-router";
import { Show, type Component, type JSX } from "solid-js";
import { GoogleLogo, WechatLogo } from "~/components/icons/Phosphor";

// This is the main layout for all /auth/* routes.
// The <Outlet /> component will render the specific child route (login, signup, etc.)
export function AuthLayout() {
  return (
    <main class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div class="w-full max-w-md space-y-8">
        <div>
          <h2 class="mt-6 text-center text-3xl font-bold tracking-tight text-gray-900">
            Jingdezhen Ceramics Platform
          </h2>
        </div>
        <div class="bg-white py-8 px-4 shadow sm:rounded-lg sm:px-10">
          <Outlet />
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/auth/_layout")({
  component: AuthLayout,
});

// --- Shared Components for Auth Forms ---

export const SocialLogins: Component<{ verb?: string }> = (props) => {
  const verb = () => props.verb || "Continue";
  return (
    <div>
      <div class="relative mt-6">
        <div class="absolute inset-0 flex items-center">
          <div class="w-full border-t border-gray-300" />
        </div>
        <div class="relative flex justify-center text-sm">
          <span class="bg-white px-2 text-gray-500">Or {verb()} with</span>
        </div>
      </div>

      <div class="mt-6 grid grid-cols-2 gap-3">
        <div>
          <button class="social-btn">
            <GoogleLogo class="h-5 w-5" />
            <span>Google</span>
          </button>
        </div>
        <div>
          <button class="social-btn">
            <WechatLogo class="h-5 w-5" />
            <span>WeChat</span>
          </button>
        </div>
      </div>
    </div>
  );
};

interface FormFieldProps {
  label: string;
  name: string;
  type: string;
  value: string;
  onInput: JSX.EventHandler<HTMLInputElement, InputEvent>;
  error?: string | null;
}

export const FormField: Component<FormFieldProps> = (props) => {
  return (
    <div>
      <label for={props.name} class="block text-sm font-medium text-gray-700">
        {props.label}
      </label>
      <div class="mt-1">
        <input
          id={props.name}
          name={props.name}
          type={props.type}
          autocomplete={props.name}
          required
          value={props.value}
          onInput={props.onInput}
          class="form-input"
          classList={{ "border-red-500": !!props.error }}
        />
        <Show when={props.error}>
          <p class="mt-2 text-sm text-red-600">{props.error}</p>
        </Show>
      </div>
    </div>
  );
};

interface MessageCardProps {
  type: "success" | "error";
  title: string;
  message: string;
}

export const MessageCard: Component<MessageCardProps> = (props) => {
  return (
    <div
      class="rounded-md p-4"
      classList={{
        "bg-green-50": props.type === "success",
        "bg-red-50": props.type === "error",
      }}
    >
      <div class="flex">
        <div class="ml-3">
          <h3
            class="text-sm font-medium"
            classList={{
              "text-green-800": props.type === "success",
              "text-red-800": props.type === "error",
            }}
          >
            {props.title}
          </h3>
          <div
            class="mt-2 text-sm"
            classList={{
              "text-green-700": props.type === "success",
              "text-red-700": props.type === "error",
            }}
          >
            <p>{props.message}</p>
          </div>
        </div>
      </div>
    </div>
  );
};
