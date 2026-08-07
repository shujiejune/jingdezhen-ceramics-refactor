import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createMemo, createSignal } from "solid-js";
import { PageHeader } from "~/components/layout/PageHeader";
import {
  ArrowFatUp,
  ChatCircleDots,
  Funnel,
  PushPin,
  CaretDown,
  CaretUp,
  MagnifyingGlass,
} from "~/components/icons/Phosphor";
import { formatLastActivity } from "~/lib/utils";
import { Link } from "@tanstack/solid-router";
import * as Select from "@kobalte/core/select";
import * as Pagination from "@kobalte/core/pagination";
import { z } from "zod";
import type { Component, JSX } from "solid-js";
import type { ForumPost } from "~/lib/types";

// --- Data, Types, and Helper Functions ---

const categories = [
  "All",
  "Events",
  "Discussion",
  "How To",
  "Showcase",
  "Feedback",
] as const;
const sortOptions = [
  "Latest Activity",
  "Newest",
  "Oldest",
  "Most Likes",
  "Most Comments",
] as const;
const allTags = [
  "blue-and-white",
  "celadon",
  "glazing",
  "technique",
  "history",
  "modern-art",
  "exhibition",
  "question",
];

type Category = (typeof categories)[number];
type SortOption = (typeof sortOptions)[number];

// Mock Data
const now = new Date("2025-08-21T10:26:00-07:00");
const allPosts: ForumPost[] = [
  {
    id: 1,
    title: "Glazing Techniques Summit Next Month!",
    authorNickname: "Admin",
    authorAvatarUrl: "https://placehold.co/40x40/E2E8F0/4A5568?text=A",
    tags: [
      { id: 1, name: "exhibition", color: "bg-purple-100 text-purple-800" },
      { id: 2, name: "technique", color: "bg-blue-100 text-blue-800" },
    ],
    categoryName: "Events",
    likeCount: 128,
    commentCount: 42,
    createdAt: "2025-08-20T11:00:00Z",
    lastActivityAt: "2025-08-21T17:15:00Z",
    isPinned: true,
    content:
      "Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.",
  },
  {
    id: 2,
    title: "Help Identifying This Mark",
    authorNickname: "L. Chen",
    authorAvatarUrl: "https://placehold.co/40x40/fef2f2/991b1b?text=L",
    tags: [
      { id: 3, name: "history", color: "bg-yellow-100 text-yellow-800" },
      { id: 4, name: "question", color: "bg-green-100 text-green-800" },
    ],
    categoryName: "Discussion",
    likeCount: 15,
    commentCount: 8,
    createdAt: "2025-08-21T10:00:00Z",
    lastActivityAt: "2025-08-21T16:30:00Z",
    isPinned: false,
    content:
      "Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. ",
  },
  {
    id: 3,
    title: "Showcase: My First Celadon Piece",
    authorNickname: "D. Miller",
    authorAvatarUrl: "https://placehold.co/40x40/eff6ff/3730a3?text=D",
    tags: [
      { id: 1, name: "exhibition", color: "bg-purple-100 text-purple-800" },
      { id: 2, name: "technique", color: "bg-blue-100 text-blue-800" },
    ],
    categoryName: "Showcase",
    likeCount: 88,
    commentCount: 19,
    createdAt: "2025-08-20T09:00:00Z",
    lastActivityAt: "2025-08-21T15:05:00Z",
    isPinned: false,
    content:
      "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
  },
  {
    id: 4,
    title: "How To: Properly Wedge Clay for Throwing",
    authorNickname: "ClayMaster",
    authorAvatarUrl: "https://placehold.co/40x40/f0fdf4/166534?text=C",
    tags: [{ id: 2, name: "technique", color: "bg-blue-100 text-blue-800" }],
    categoryName: "How To",
    likeCount: 250,
    commentCount: 67,
    createdAt: "2025-07-15T12:00:00Z",
    lastActivityAt: "2025-08-20T22:10:00Z",
    isPinned: false,
    content:
      "Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.",
  },
  {
    id: 5,
    title: "Feedback on the New Portfolio Layout",
    authorNickname: "Jane",
    authorAvatarUrl: "https://placehold.co/40x40/fefce8/854d0e?text=J",
    tags: [],
    categoryName: "Feedback",
    likeCount: 5,
    commentCount: 2,
    createdAt: "2024-06-10T18:00:00Z",
    lastActivityAt: "2025-08-19T11:00:00Z",
    isPinned: false,
    content:
      "Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.",
  },
  // ... add more posts for pagination
];

// --- Data Fetching Logic ---

// This object will hold the response from the backend
interface PostsApiResponse {
  posts: ForumPost[];
  totalCount: number;
}

// This function simulates calling backend API
const fetchPosts = async (filters: {
  page: number;
  category: Category;
  sort: SortOption;
  q?: string;
  tag?: string;
}): Promise<PostsApiResponse> => {
  console.log("Fetching posts with filters:", filters);
  // In a real app, you would build a URL query string from the filters
  // const response = await fetch(`/api/posts?page=${filters.page}&...`);
  // const data = await response.json();
  // return data;

  // For now, we return mock data
  await new Promise((r) => setTimeout(r, 300)); // Simulate network delay
  return {
    posts: allPosts,
    totalCount: 5,
  };
};

// --- Route Definition with Search Param Validation ---

const forumSearchSchema = z.object({
  page: z.number().int().positive().default(1).catch(1),
  category: z.enum(categories).default("All").catch("All"),
  sort: z.enum(sortOptions).default("Latest Activity").catch("Latest Activity"),
  q: z.string().optional(),
  view: z.enum(["posts", "tags"]).default("posts").catch("posts"),
  tag: z.string().optional(),
});

export const Route = createFileRoute("/forum/")({
  validateSearch: (search) => forumSearchSchema.parse(search),
  component: ForumPage,
});

// --- Main Page Component ---

function ForumPage() {
  const searchParams = Route.useSearch();
  // This is the core of our data-driven page.
  // `createQuery` will automatically re-fetch data whenever `searchParams()` changes.
  const postsQuery = useQuery(() => ({
    queryKey: ["forum-posts", searchParams()], // Unique key for this set of filters
    queryFn: () => fetchPosts(searchParams()),
    placeholderData: (prev) => prev, // Keep showing old data while new data is loading
  }));

  return (
    <div class="bg-gray-50 min-h-full flex flex-col">
      <PageHeader
        title="Forum"
        subtitle="Connect with fellow ceramic enthusiasts, share your work, and ask questions."
      />
      <div class="forum-layout flex-grow">
        <Sidebar />
        <main class="main-content">
          <Suspense fallback={<p>Loading...</p>}>
            <Show
              when={searchParams().view === "posts"}
              fallback={<TagCloud />}
            >
              <MainContent
                posts={postsQuery.data?.posts ?? []}
                totalCount={postsQuery.data?.totalCount ?? 0}
              />
            </Show>
          </Suspense>
        </main>
      </div>
    </div>
  );
}

// --- Sub-Components ---

const Sidebar: Component = () => {
  const searchParams = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const handleCategoryClick = (category: Category) => {
    navigate({
      search: (prev) => ({ ...prev, category, page: 1, view: "posts" }),
    });
  };

  const showTagsView = () => {
    navigate({ search: (prev) => ({ ...prev, view: "tags" }) });
  };

  return (
    <aside class="sidebar">
      <div class="sidebar-section">
        <button class="sidebar-title-button" onClick={showTagsView}>
          TAGS
        </button>
      </div>

      <div class="sidebar-section">
        <h3 class="sidebar-title">CATEGORIES</h3>
        <div class="category-list">
          <For each={categories}>
            {(cat) => (
              <button
                class="category-button"
                data-active={
                  searchParams().category === cat &&
                  searchParams().view === "posts"
                }
                onClick={() => handleCategoryClick(cat)}
              >
                {cat}
              </button>
            )}
          </For>
        </div>
      </div>
    </aside>
  );
};

const MainContent: Component<{ posts: ForumPost[]; totalCount: number }> = (
  props,
) => {
  const searchParams = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });
  const postsPerPage = 20;
  const totalPages = () => Math.ceil(props.totalCount / postsPerPage);

  const handlePageChange = (page: number) => {
    navigate({ search: (prev) => ({ ...prev, page }) });
  };

  // Pinned posts would likely be a separate API call in a real app
  const pinnedPosts: ForumPost[] = []; // Assuming pinned posts are handled separately

  return (
    <>
      <FilterBar />
      <div class="post-list">
        <For each={pinnedPosts}>
          {(post) => <PostListItem post={post} isPinned={true} />}
        </For>
        <For each={props.posts}>{(post) => <PostListItem post={post} />}</For>
      </div>
      <Show when={totalPages() > 1}>
        <Pagination.Root
          class="pagination"
          count={totalPages()}
          page={searchParams().page}
          onPageChange={handlePageChange}
          itemComponent={(props) => (
            <Pagination.Item page={props.page} class="page-item">
              {props.page}
            </Pagination.Item>
          )}
          ellipsisComponent={() => (
            <Pagination.Ellipsis class="page-ellipsis">...</Pagination.Ellipsis>
          )}
        >
          <Pagination.Previous class="page-prev">Prev</Pagination.Previous>
          <Pagination.Next class="page-next">Next</Pagination.Next>
        </Pagination.Root>
      </Show>
    </>
  );
};

const FilterBar: Component = () => {
  const searchParams = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const handleSortChange = (sort: SortOption | null) => {
    if (sort) {
      navigate({ search: (prev) => ({ ...prev, sort, page: 1 }) });
    }
  };

  const handleSearch = (e: Event) => {
    const value = (e.currentTarget as HTMLInputElement).value;
    navigate({
      search: (prev) => ({ ...prev, q: value || undefined, page: 1 }),
    });
  };

  return (
    <div class="filter-bar">
      <div class="search-wrapper">
        <MagnifyingGlass size={20} class="search-icon" />
        <input
          type="search"
          placeholder="Search posts..."
          class="search-bar"
          onInput={handleSearch}
          value={searchParams().q || ""}
        />
      </div>
      <div class="filter-actions">
        <Select.Root<SortOption>
          options={[...sortOptions]}
          value={searchParams().sort}
          onChange={handleSortChange}
          itemComponent={(props) => (
            <Select.Item item={props.item} class="select-item">
              <Select.ItemLabel>{props.item.rawValue}</Select.ItemLabel>
            </Select.Item>
          )}
        >
          <Select.Trigger class="select-trigger">
            <Funnel size={16} />
            <span class="sort-by-text">Sort by:</span>
            <Select.Value<SortOption>>
              {(state) => state.selectedOption()}
            </Select.Value>
            <Select.Icon class="select-caret">
              <CaretDown class="caret-down" />
              <CaretUp class="caret-up" />
            </Select.Icon>
          </Select.Trigger>
          <Select.Portal>
            <Select.Content class="select-content">
              <Select.Listbox />
            </Select.Content>
          </Select.Portal>
        </Select.Root>
        <Link to="/forum/new" class="new-post-button">
          New Post
        </Link>
      </div>
    </div>
  );
};

const TagCloud: Component = () => {
  // In a real app, tags and their counts would be fetched from an API
  const tagsWithCounts = [
    { name: "blue-and-white", count: 58 },
    { name: "celadon", count: 42 },
    { name: "glazing", count: 95 },
    { name: "technique", count: 120 },
    { name: "history", count: 30 },
    { name: "modern-art", count: 75 },
  ];

  const navigate = useNavigate({ from: Route.fullPath });
  const handleTagClick = (tagName: string) => {
    navigate({
      search: (prev) => ({ ...prev, view: "posts", tag: tagName, page: 1 }),
    });
  };

  return (
    <div class="tag-cloud-container">
      <h2 class="tag-cloud-title">Explore Tags</h2>
      <div class="tag-cloud">
        <For each={tagsWithCounts}>
          {(tag) => (
            <button
              class="tag"
              style={{
                "font-size": `${Math.min(1.2 + tag.count * 0.01, 2.5)}rem`,
              }}
              onClick={() => handleTagClick(tag.name)}
            >
              {tag.name}
            </button>
          )}
        </For>
      </div>
    </div>
  );
};

const PostListItem: Component<{ post: ForumPost; isPinned?: boolean }> = (
  props,
) => {
  return (
    <div class="post-item" data-pinned={props.isPinned}>
      <div class="post-votes">
        <ArrowFatUp size={20} />
        <span>{props.post.likeCount}</span>
      </div>
      <div class="post-main">
        <Link
          to="/forum/$postId"
          params={{ postId: props.post.id.toString() }}
          class="post-title-link"
        >
          {props.isPinned && (
            <span class="pinned-badge">
              <PushPin size={20} />
            </span>
          )}
          {props.post.title}
        </Link>
        <div class="post-tags">
          <For each={props.post.tags}>
            {(tag) => (
              <span
                class={`post-tag-capsule ${tag.color || "bg-gray-100 text-gray-800"}`}
              >
                {tag.name}
              </span>
            )}
          </For>
        </div>
      </div>
      <div class="post-author">
        <img
          src={props.post.authorAvatarUrl}
          alt={`${props.post.authorNickname}'s avatar`}
          class="author-avatar"
        />
      </div>
      <div class="post-comments">
        <ChatCircleDots size={18} />
        <span>{props.post.commentCount}</span>
      </div>
      <div class="post-activity">
        {formatLastActivity(props.post.lastActivityAt)}
      </div>
    </div>
  );
};
