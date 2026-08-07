import { createFileRoute } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { Show, Suspense, type Component } from "solid-js";

// Mock API fetching for a public user profile
const fetchPublicUserProfile = async (userId: string) => {
  console.log("Fetching public profile for user:", userId);
  await new Promise((r) => setTimeout(r, 500));
  // In a real app, you'd fetch this user's public info from your backend
  return {
    id: userId,
    nickname: `Artist-${userId.substring(0, 4)}`,
    avatarUrl: `https://placehold.co/120x120/e0e7ff/4338ca?text=${userId.substring(0, 2)}`,
    joinDate: new Date().toISOString(),
  };
};

export const Route = createFileRoute("/account/$userId")({
  component: UserProfilePage,
});

function UserProfilePage() {
  const params = Route.useParams();
  const userQuery = useQuery(() => ({
    queryKey: ["user-profile", params().userId],
    queryFn: () => fetchPublicUserProfile(params().userId),
  }));

  return (
    <div class="container mx-auto px-4 py-12">
      <Suspense fallback={<p class="text-center">Loading profile...</p>}>
        <Show when={userQuery.data} keyed>
          {(user) => (
            <div class="text-center">
              <img
                src={user.avatarUrl}
                alt="User Avatar"
                class="w-32 h-32 rounded-full mx-auto"
              />
              <h1 class="mt-4 text-3xl font-bold">{user.nickname}</h1>
              <p class="mt-2 text-gray-500">
                Member since {new Date(user.joinDate).toLocaleDateString()}
              </p>

              <div class="mt-8">
                <h2 class="text-2xl font-semibold">Public Works</h2>
                {/* In a real application, you would create another component here
                  that fetches and displays this user's public portfolio works or forum posts.
                */}
                <p class="mt-4 text-gray-600">
                  This user's public portfolio will be displayed here.
                </p>
              </div>
            </div>
          )}
        </Show>
      </Suspense>
    </div>
  );
}
