import { createFileRoute, Link, redirect } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import {
  For,
  Show,
  Suspense,
  createSignal,
  type Component,
  type JSX,
} from "solid-js";
import { z } from "zod";
import {
  AccountSidebar,
  type TabItem,
} from "~/components/layout/AccountSidebar";
import {
  Bell,
  BookOpen,
  Notebook,
  Palette,
  Swap,
  Heart,
  BookmarkSimple,
  MagnifyingGlass,
} from "~/components/icons/Phosphor";
import {
  Notification,
  EnrolledCourse,
  Note,
  UserWork,
  UserFavorite,
  UserPost,
} from "~/lib/types";

// --- Mock Auth & User Data ---
const getLoggedInUser = () => ({
  id: "user-abc",
  nickname: "CeramicMaster",
  avatarUrl: "https://placehold.co/100x100/d1fae5/065f46?text=CM",
});
const auth = {
  // Set this to `false` in your testing to see the redirect to /login in action.
  isAuthenticated: true,
};

// --- Mock Data & API Fetching ---

const fetchNotifications = async (): Promise<Notification[]> => {
  await new Promise((r) => setTimeout(r, 300));
  return [
    {
      id: 1,
      actorUser: { id: "user-2", nickname: "ClayFan", avatarUrl: "" },
      notificationType: "comment",
      entityType: "post",
      entityId: 123,
      entityTitle: "Glazing Techniques",
      message: "replied to your post.",
      isRead: false,
      createdAt: "2025-09-17T10:30:00Z",
    },
    {
      id: 2,
      actorUser: { id: "user-3", nickname: "PotterHead", avatarUrl: "" },
      notificationType: "mention",
      entityType: "artwork",
      entityId: 3,
      entityTitle: "Vase #3",
      message: "mentioned you in a note.",
      isRead: true,
      createdAt: "2025-09-16T18:00:00Z",
    },
    {
      id: 3,
      notificationType: "system",
      entityType: "course",
      entityId: 4,
      entityTitle: "Advanced Wheel Throwing",
      message: "Your new course has been approved.",
      isRead: true,
      createdAt: "2025-09-15T12:00:00Z",
    },
  ];
};
const fetchEnrolledCourses = async (): Promise<EnrolledCourse[]> => {
  await new Promise((r) => setTimeout(r, 300));
  return [
    {
      id: 1,
      title: "Introduction to Hand-Building",
      thumbnailUrl: "https://placehold.co/400x300/e0f2fe/0891b2?text=Course+1",
      progress: 75,
    },
    {
      id: 2,
      title: "Glaze Chemistry 101",
      thumbnailUrl: "https://placehold.co/400x300/dbeafe/1e40af?text=Course+2",
      progress: 40,
    },
  ];
};
const fetchMyNotes = async (): Promise<Note[]> => {
  await new Promise((r) => setTimeout(r, 300));
  return [
    {
      id: 1,
      title: "Cobalt Blue observations",
      content:
        "The 'heaping and piling' is very pronounced here. Need to compare with Xuande period examples.",
      entityType: "artwork",
      entityId: 1,
      entityTitle: "Blue-and-white Plate",
      createdAt: "2025-09-10T11:00:00Z",
      updatedAt: "2025-09-12T15:45:00Z",
    },
    {
      id: 2,
      title: "Dragon Motif study",
      content: "The three-clawed dragon is typical for this period.",
      entityType: "artwork",
      entityId: 1,
      entityTitle: "Blue-and-white Plate",
      createdAt: "2025-09-01T18:20:00Z",
      updatedAt: "2025-09-01T18:20:00Z",
    },
  ];
};
const fetchMyWorks = async (): Promise<UserWork[]> => {
  await new Promise((r) => setTimeout(r, 300));
  return [
    {
      id: 1,
      title: "Spiral Vase",
      thumbnailUrl: "https://placehold.co/400x400/fff7ed/c2410c?text=Work+1",
      upvotesCount: 128,
    },
    {
      id: 2,
      title: "Tea Bowl Set",
      thumbnailUrl: "https://placehold.co/400x400/fef2f2/b91c1c?text=Work+2",
      upvotesCount: 95,
    },
  ];
};
const fetchMyPosts = async (): Promise<UserPost[]> => {
  await new Promise((r) => setTimeout(r, 300));
  return [
    {
      id: 123,
      title: "Glazing Techniques for Beginners",
      categoryName: "How To",
      createdAt: "2025-08-20T14:00:00Z",
      commentCount: 12,
      likeCount: 45,
    },
    {
      id: 124,
      title: "Showcase: My latest wood-fired pieces",
      categoryName: "Showcase",
      createdAt: "2025-07-11T09:30:00Z",
      commentCount: 5,
      likeCount: 33,
    },
  ];
};
const fetchMyFavorites = async (): Promise<UserFavorite[]> => {
  await new Promise((r) => setTimeout(r, 300));
  return [
    {
      id: 1,
      title: "Blue-and-white Plate",
      thumbnailUrl: "https://placehold.co/400x600/e0f2fe/0891b2?text=Fav+1",
      artistName: "Unknown",
      period: "Ming Yongle",
    },
    {
      id: 2,
      title: "Famille Rose Vase",
      thumbnailUrl: "https://placehold.co/400x600/fce7f3/831843?text=Fav+2",
      artistName: "Unknown",
      period: "Qing Yongzheng",
    },
  ];
};
const fetchMySaves = async (): Promise<UserPost[]> => {
  return fetchMyPosts(); // For demo, assume saved posts are same as user's posts
};

// You would also have fetch functions for Works, Posts, Favorites, and Saves
// For brevity, their panels will show placeholder content.

// --- Route Definition ---

const accountSearchSchema = z.object({
  tab: z
    .enum([
      "notifications",
      "enrolled-courses",
      "my-notes",
      "my-works",
      "my-posts",
      "my-favorites",
      "my-saves",
    ])
    .default("notifications")
    .catch("notifications"),
});

export type AccountTab = z.infer<typeof accountSearchSchema>["tab"];

export const Route = createFileRoute("/account/")({
  // This is a protected route. If the user is not authenticated,
  // they will be redirected to the login page before this component loads.
  beforeLoad: async ({ location }) => {
    if (!auth.isAuthenticated) {
      throw redirect({
        to: "/auth/login",
        search: {
          // Pass the current page as a redirect param so the user can be returned here after login
          redirect: location.pathname,
        },
      });
    }
  },
  validateSearch: (search) => accountSearchSchema.parse(search),
  loader: ({ location }) => {
    const validatedSearchParams = accountSearchSchema.parse(location.search);
    return { searchParams: validatedSearchParams };
  },
  component: AccountPage,
});

// --- Main Page Component ---

function AccountPage() {
  const loaderData = Route.useLoaderData();
  const activeTab = () => loaderData().searchParams.tab;
  const user = getLoggedInUser();

  const tabs: TabItem<AccountTab>[] = [
    { id: "notifications", label: "Notifications", icon: Bell },
    { id: "enrolled-courses", label: "Enrolled Courses", icon: BookOpen },
    { id: "my-notes", label: "My Notes", icon: Notebook },
    { id: "my-works", label: "My Works", icon: Palette },
    { id: "my-posts", label: "My Posts", icon: Swap },
    { id: "my-favorites", label: "My Favorites", icon: Heart },
    { id: "my-saves", label: "My Saves", icon: BookmarkSimple },
  ];

  return (
    <div class="container mx-auto px-4 py-8">
      <div class="flex flex-col md:flex-row gap-8">
        <AccountSidebar user={user} tabs={tabs} activeTab={activeTab()} />
        <main class="flex-grow">
          <Suspense
            fallback={
              <div class="w-full h-64 bg-gray-200 rounded-lg animate-pulse" />
            }
          >
            <Show when={activeTab() === "notifications"}>
              <NotificationsPanel />
            </Show>
            <Show when={activeTab() === "enrolled-courses"}>
              <EnrolledCoursesPanel />
            </Show>
            <Show when={activeTab() === "my-notes"}>
              <MyNotesPanel />
            </Show>
            <Show when={activeTab() === "my-works"}>
              <MyWorksPanel />
            </Show>
            <Show when={activeTab() === "my-posts"}>
              <MyPostsPanel />
            </Show>
            <Show when={activeTab() === "my-favorites"}>
              <MyFavoritesPanel />
            </Show>
            <Show when={activeTab() === "my-saves"}>
              <MySavesPanel />
            </Show>
          </Suspense>
        </main>
      </div>
    </div>
  );
}

// --- Panel Components ---

const PanelHeader: Component<{ title: string; children?: any }> = (props) => (
  <div class="flex justify-between items-center mb-6">
    <h2 class="text-2xl font-bold">{props.title}</h2>
    <div>{props.children}</div>
  </div>
);

const EntityLink: Component<{
  type?: "post" | "artwork" | "course" | "course_chapter";
  id?: number;
  parentId?: number;
  class?: string;
  children: JSX.Element;
}> = (props) => {
  const commonProps = { class: props.class, target: "_blank" };

  if (props.type === "post" && props.id) {
    return (
      <Link
        to="/forum/posts/$postId"
        params={{ postId: props.id.toString() }}
        {...commonProps}
      >
        {props.children}
      </Link>
    );
  }
  if (props.type === "artwork" && props.id) {
    return (
      <Link
        to="/gallery/artworks/$artworkId"
        params={{ artworkId: props.id.toString() }}
        {...commonProps}
      >
        {props.children}
      </Link>
    );
  }
  if (
    (props.type === "course" || props.type === "course_chapter") &&
    props.id
  ) {
    const courseId = props.parentId ?? props.id;
    return (
      <Link
        to="/course/$courseId"
        params={{ courseId: courseId.toString() }}
        {...commonProps}
      >
        {props.children}
      </Link>
    );
  }
  // Fallback for missing data
  return <span class={props.class}>{props.children}</span>;
};

const NotificationsPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-notifications"],
    queryFn: fetchNotifications,
  }));

  return (
    <div>
      <PanelHeader title="Notifications" />
      <div class="border rounded-lg bg-white overflow-hidden">
        <table class="w-full text-sm text-left">
          <thead class="bg-gray-50 border-b">
            <tr>
              <th class="px-6 py-3 font-medium">Content</th>
              <th class="px-6 py-3 font-medium">Date</th>
            </tr>
          </thead>
          <tbody>
            <For each={query.data}>
              {(notification) => (
                <tr class="border-b hover:bg-gray-50">
                  <td class="px-6 py-4">
                    <div class="flex items-center gap-2">
                      <Show when={!notification.isRead}>
                        <div class="w-2 h-2 bg-blue-500 rounded-full flex-shrink-0" />
                      </Show>
                      <span>
                        <Show when={notification.actorUser}>
                          <Link
                            to={"/account/$userId"}
                            params={{ userId: notification.actorUser!.id }}
                            search={{ tab: "notifications" }}
                            target="_blank"
                            class="font-semibold text-gray-800 hover:underline"
                          >
                            {notification.actorUser!.nickname}
                          </Link>
                        </Show>{" "}
                        {notification.message}{" "}
                        <Show when={notification.entityTitle}>
                          <EntityLink
                            type={notification.entityType}
                            id={notification.entityId}
                            class="font-semibold text-blue-600 hover:underline"
                          >
                            '{notification.entityTitle}'
                          </EntityLink>
                        </Show>
                      </span>
                    </div>
                  </td>
                  <td class="px-6 py-4 text-gray-500">
                    {new Date(notification.createdAt).toLocaleDateString()}
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </div>
  );
};

const EnrolledCoursesPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-courses"],
    queryFn: fetchEnrolledCourses,
  }));

  return (
    <div>
      <PanelHeader title="Enrolled Courses" />
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
        <For each={query.data}>
          {(course) => (
            <div class="border rounded-lg bg-white overflow-hidden shadow-sm">
              <img
                src={course.thumbnailUrl}
                alt={course.title}
                class="w-full h-40 object-cover"
              />
              <div class="p-4">
                <h3 class="font-semibold truncate">{course.title}</h3>
                <div class="mt-3 w-full bg-gray-200 rounded-full h-2.5">
                  <div
                    class="bg-blue-600 h-2.5 rounded-full"
                    style={{ width: `${course.progress}%` }}
                  />
                </div>
                <p class="text-xs text-gray-500 mt-1">
                  {course.progress}% complete
                </p>
              </div>
            </div>
          )}
        </For>
      </div>
    </div>
  );
};

const MyNotesPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-notes"],
    queryFn: fetchMyNotes,
  }));
  const [searchTerm, setSearchTerm] = createSignal("");

  const createExcerpt = (content: string, length = 100) => {
    const strippedContent = content.replace(/<[^>]+>|#+\s/g, ""); // Remove HTML/Markdown headers
    return strippedContent.length > length
      ? strippedContent.substring(0, length) + "..."
      : strippedContent;
  };

  const filteredNotes = () => {
    const lowerSearch = searchTerm().toLowerCase();
    if (!lowerSearch) return query.data;
    return query.data?.filter(
      (note) =>
        note.title.toLowerCase().includes(lowerSearch) ||
        note.entityTitle?.toLowerCase().includes(lowerSearch) ||
        note.content.toLowerCase().includes(lowerSearch),
    );
  };

  return (
    <div>
      <PanelHeader title="My Notes">
        <div class="relative w-64">
          <input
            type="text"
            placeholder="Search notes..."
            value={searchTerm()}
            onInput={(e) => setSearchTerm(e.currentTarget.value)}
            class="form-input w-full pl-10"
          />
          <MagnifyingGlass class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        </div>
      </PanelHeader>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
        <For each={filteredNotes()}>
          {(note) => (
            <div class="border rounded-lg bg-white p-4 shadow-sm">
              <h3 class="font-semibold">{note.title}</h3>
              <p class="text-sm text-gray-600 mt-1 italic">
                on{" "}
                <EntityLink
                  type={note.entityType}
                  id={note.entityId}
                  parentId={note.parentEntityId}
                  class="text-blue-600 hover:underline"
                >
                  {note.entityTitle}
                </EntityLink>
              </p>
              <p class="text-sm text-gray-500 mt-2 line-clamp-2">
                {createExcerpt(note.content)}
              </p>
            </div>
          )}
        </For>
      </div>
    </div>
  );
};

const MyWorksPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-works"],
    queryFn: fetchMyWorks,
  }));
  return (
    <div>
      <PanelHeader title="My Works" />
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
        <For each={query.data}>
          {(work) => (
            <Link
              to={"/portfolio/works/$workId"}
              params={{ workId: work.id.toString() }}
              class="border rounded-lg bg-white overflow-hidden shadow-sm group"
            >
              <img
                src={work.thumbnailUrl}
                alt={work.title}
                class="w-full h-48 object-cover transition-transform duration-200 group-hover:scale-105"
              />
              <div class="p-4 flex justify-between items-center">
                <h3 class="font-semibold truncate">{work.title}</h3>
                <div class="flex items-center gap-1 text-sm text-gray-500">
                  <Heart fill="red" class="text-red-400" /> {work.upvotesCount}
                </div>
              </div>
            </Link>
          )}
        </For>
      </div>
    </div>
  );
};

const MyPostsPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-posts"],
    queryFn: fetchMyPosts,
  }));
  return (
    <div>
      <PanelHeader title="My Posts" />
      <div class="border rounded-lg bg-white overflow-hidden">
        <table class="w-full text-sm text-left">
          <thead class="bg-gray-50 border-b">
            <tr class="text-gray-600">
              <th class="px-6 py-3 font-medium">Title</th>
              <th class="px-6 py-3 font-medium">Stats</th>
              <th class="px-6 py-3 font-medium">Date</th>
            </tr>
          </thead>
          <tbody>
            <For each={query.data}>
              {(post) => (
                <tr class="border-b hover:bg-gray-50">
                  <td class="px-6 py-4">
                    <Link
                      to={"/forum/posts/$postId"}
                      params={{ postId: post.id.toString() }}
                      class="font-semibold text-blue-600 hover:underline"
                    >
                      {post.title}
                    </Link>
                    <p class="text-xs text-gray-500">{post.categoryName}</p>
                  </td>
                  <td class="px-6 py-4 text-gray-600">
                    {post.commentCount} comments / {post.likeCount} upvotes
                  </td>
                  <td class="px-6 py-4 text-gray-500">
                    {new Date(post.createdAt).toLocaleDateString()}
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </div>
  );
};

const MyFavoritesPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-favorites"],
    queryFn: fetchMyFavorites,
  }));
  return (
    <div>
      <PanelHeader title="My Favorites" />
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
        <For each={query.data}>
          {(fav) => (
            <Link
              to={"/gallery/artworks/$artworkId"}
              params={{ artworkId: fav.id.toString() }}
              class="border rounded-lg bg-white overflow-hidden shadow-sm group"
            >
              <img
                src={fav.thumbnailUrl}
                alt={fav.title}
                class="w-full h-64 object-cover transition-transform duration-200 group-hover:scale-105"
              />
              <div class="p-4">
                <h3 class="font-semibold truncate">{fav.title}</h3>
                <p class="text-sm text-gray-500">{fav.period}</p>
              </div>
            </Link>
          )}
        </For>
      </div>
    </div>
  );
};

const MySavesPanel: Component = () => {
  const query = useQuery(() => ({
    queryKey: ["account-saves"],
    queryFn: fetchMySaves,
  }));
  return (
    <div>
      <PanelHeader title="My Saved Posts" />
      {/* This reuses the same table structure as MyPostsPanel */}
      <div class="border rounded-lg bg-white overflow-hidden">
        <table class="w-full text-sm text-left">
          <thead class="bg-gray-50 border-b">
            <tr class="text-gray-600">
              <th class="px-6 py-3 font-medium">Title</th>
              <th class="px-6 py-3 font-medium">Stats</th>
              <th class="px-6 py-3 font-medium">Date</th>
            </tr>
          </thead>
          <tbody>
            <For each={query.data}>
              {(post) => (
                <tr class="border-b hover:bg-gray-50">
                  <td class="px-6 py-4">
                    <Link
                      to={"/forum/posts/$postId"}
                      params={{ postId: post.id.toString() }}
                      class="font-semibold text-blue-600 hover:underline"
                    >
                      {post.title}
                    </Link>
                    <p class="text-xs text-gray-500">{post.categoryName}</p>
                  </td>
                  <td class="px-6 py-4 text-gray-600">
                    {post.commentCount} comments / {post.likeCount} upvotes
                  </td>
                  <td class="px-6 py-4 text-gray-500">
                    {new Date(post.createdAt).toLocaleDateString()}
                  </td>
                </tr>
              )}
            </For>
          </tbody>
        </table>
      </div>
    </div>
  );
};
