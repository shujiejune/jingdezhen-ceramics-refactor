import { createFileRoute } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal } from "solid-js";
import { marked } from "marked";
import * as DropdownMenu from "@kobalte/core/dropdown-menu";
import type { Component, JSX } from "solid-js";
import type { ForumPost, Tag } from "~/lib/types";
import { formatLastActivity } from "~/lib/utils";
import { MarkdownEditor } from "~/components/shared/MarkdownEditor";
import {
  ThumbsUp,
  BookmarkSimple,
  DotsThree,
} from "~/components/icons/Phosphor";

// --- Data Fetching and Types ---

interface ForumComment {
  id: number;
  authorNickname: string;
  authorAvatarUrl?: string;
  content: string;
  htmlContent: string;
  createdAt: string;
  likeCount: number;
  replies: ForumComment[]; // For nesting
  isOwned: boolean; // To determine if "Edit" shows
}

interface PostDetails extends ForumPost {
  htmlContent: string;
  viewCount: number;
  comments: ForumComment[];
  isOwned: boolean; // To determine if "Edit" shows
}

// Mock function to fetch a single post and its comments
const fetchPostDetails = async (postId: string): Promise<PostDetails> => {
  console.log(`Fetching post details for ID: ${postId}`);
  await new Promise((r) => setTimeout(r, 500));

  const rawContent = `I found this piece at a local market and was struck by the mark on the bottom. I've attached a photo below.

  ![Mark](https://placehold.co/600x300/e0e0e0/4d4d4d?text=Ceramic+Mark)

  It doesn't look like any of the standard Ming or Qing dynasty marks I'm familiar with.`;

  const comment1Content =
    "That's a fascinating mark! It has some characteristics of provincial kilns from the late Qing period.";
  const reply1Content =
    "That's a great insight, thank you! I was thinking it might be a provincial piece.";
  const comment2Content =
    "I agree with ClayMaster. It could also be a modern reproduction made in an older style.";

  // In a real app, this would be an API call
  return {
    id: parseInt(postId),
    title: "Help Identifying This Mark",
    authorNickname: "L. Chen",
    authorAvatarUrl: "https://placehold.co/40x40/fef2f2/991b1b?text=L",
    tags: [
      { id: 3, name: "history", color: "bg-yellow-100 text-yellow-800" },
      { id: 4, name: "question", color: "bg-red-100 text-red-800" },
    ],
    categoryName: "Discussion",
    likeCount: 15,
    commentCount: 3,
    createdAt: "2025-08-21T10:00:00Z",
    lastActivityAt: "2025-08-21T16:30:00Z",
    isPinned: false,
    content: rawContent,
    htmlContent: await marked.parse(rawContent),
    viewCount: 258,
    isOwned: true, // Mock: user owns this post
    comments: [
      {
        id: 101,
        authorNickname: "ClayMaster",
        authorAvatarUrl: "https://placehold.co/40x40/f0fdf4/166534?text=C",
        content: comment1Content,
        htmlContent: await marked.parse(comment1Content),
        createdAt: "2025-08-21T12:45:00Z",
        likeCount: 5,
        isOwned: false,
        replies: [
          {
            id: 103,
            authorNickname: "L. Chen",
            authorAvatarUrl: "https://placehold.co/40x40/fef2f2/991b1b?text=L",
            content: reply1Content,
            htmlContent: await marked.parse(reply1Content),
            createdAt: "2025-08-21T13:10:00Z",
            likeCount: 2,
            replies: [],
            isOwned: true,
          },
        ],
      },
      {
        id: 102,
        authorNickname: "D. Miller",
        authorAvatarUrl: "https://placehold.co/40x40/eff6ff/3730a3?text=D",
        content: comment2Content,
        htmlContent: await marked.parse(comment2Content),
        createdAt: "2025-08-21T14:20:00Z",
        likeCount: 1,
        replies: [],
        isOwned: false,
      },
    ],
  };
};

// --- Route Definition with Loader ---

export const Route = createFileRoute("/forum/posts/$postId")({
  loader: ({ params: { postId } }) => fetchPostDetails(postId),
  component: PostDetailPage,
});

// --- Main Page Component ---

function PostDetailPage() {
  const postQuery = useQuery(() => ({
    queryKey: ["forum-post", Route.useParams()().postId],
    queryFn: () => fetchPostDetails(Route.useParams()().postId),
  }));

  return (
    <div class="bg-gray-50 min-h-full">
      <main class="max-w-4xl mx-auto py-8 px-4">
        <Suspense fallback={<p>Loading post...</p>}>
          <Show when={postQuery.data}>
            {(post) => (
              <>
                {/* Post Header */}
                <header class="mb-4">
                  <h1 class="text-3xl font-bold text-gray-900">
                    {post().title}
                  </h1>
                  <div class="mt-2 flex flex-wrap gap-2">
                    <For each={post().tags}>
                      {(tag) => (
                        <span
                          class={`post-tag-capsule ${tag.color || "bg-gray-100 text-gray-800"}`}
                        >
                          {tag.name}
                        </span>
                      )}
                    </For>
                  </div>
                </header>

                {/* Main Post Card */}
                <PostCard post={post()} />

                {/* Comments Section */}
                <section class="mt-8">
                  <div class="mb-6 text-lg font-semibold text-gray-700">
                    {post().commentCount} Comments, {post().viewCount} Views
                  </div>
                  <div class="space-y-6">
                    <For each={post().comments}>
                      {(comment) => <CommentCard comment={comment} />}
                    </For>
                  </div>
                </section>

                {/* Main Comment Editor */}
                <section class="mt-12">
                  <h3 class="text-xl font-semibold text-gray-800 mb-4">
                    Suggest an answer
                  </h3>
                  <MarkdownEditor placeholder="Contribute to the discussion..." />
                </section>
              </>
            )}
          </Show>
        </Suspense>
      </main>
    </div>
  );
}

// --- Reusable Sub-Components ---

const PostCard: Component<{ post: PostDetails }> = (props) => {
  return (
    <div class="post-card">
      <CardHeader
        authorNickname={props.post.authorNickname}
        authorAvatarUrl={props.post.authorAvatarUrl}
        categoryName={props.post.categoryName}
        createdAt={props.post.createdAt}
        isOwnContent={props.post.isOwned}
      />
      <div class="card-content prose" innerHTML={props.post.htmlContent} />
      <CardFooter isPost={true} />
    </div>
  );
};

const CommentCard: Component<{ comment: ForumComment }> = (props) => {
  const [isReplying, setIsReplying] = createSignal(false);

  return (
    <div class="flex gap-4">
      <img
        src={
          props.comment.authorAvatarUrl ||
          "https://placehold.co/40x40/E2E8F0/4A5568?text=?"
        }
        alt=""
        class="w-10 h-10 rounded-full mt-1 flex-shrink-0"
      />
      <div class="flex-grow">
        <div class="comment-card">
          <CardHeader
            authorNickname={props.comment.authorNickname}
            createdAt={props.comment.createdAt}
            isOwnContent={props.comment.isOwned}
          />
          <div
            class="card-content prose-sm"
            innerHTML={props.comment.htmlContent}
          />
          <CardFooter isPost={false} onReplyClick={() => setIsReplying(true)} />
        </div>

        {/* Nested Replies */}
        <Show when={props.comment.replies.length > 0}>
          <div class="mt-4 space-y-4">
            <For each={props.comment.replies}>
              {(reply) => <CommentCard comment={reply} />}
            </For>
          </div>
        </Show>

        {/* Reply Editor */}
        <Show when={isReplying()}>
          <div class="mt-4">
            <MarkdownEditor
              placeholder="Write a reply..."
              onCancel={() => setIsReplying(false)}
            />
          </div>
        </Show>
      </div>
    </div>
  );
};

const CardHeader: Component<{
  authorNickname: string;
  authorAvatarUrl?: string;
  categoryName?: string;
  createdAt: string;
  isOwnContent: boolean;
}> = (props) => {
  return (
    <div class="card-header">
      <div class="flex items-center gap-2">
        <Show when={props.authorAvatarUrl}>
          <img
            src={props.authorAvatarUrl}
            alt=""
            class="w-6 h-6 rounded-full"
          />
        </Show>
        <span class="font-semibold text-gray-800">{props.authorNickname}</span>
        <Show when={props.categoryName}>
          <span class="text-gray-500">posted in {props.categoryName}</span>
        </Show>
        <span class="text-gray-500">
          on {formatLastActivity(props.createdAt)}
        </span>
      </div>
      <CardMenu isOwnContent={props.isOwnContent} />
    </div>
  );
};

const CardFooter: Component<{ isPost: boolean; onReplyClick?: () => void }> = (
  props,
) => {
  return (
    <div class="card-footer">
      <div class="flex items-center gap-4">
        <button class="action-button">
          <ThumbsUp size={18} /> Like
        </button>
        <Show when={props.isPost}>
          <button class="action-button">
            <BookmarkSimple size={18} /> Save
          </button>
        </Show>
      </div>
      <Show when={!props.isPost}>
        <button class="reply-button" onClick={props.onReplyClick}>
          Write a reply
        </button>
      </Show>
    </div>
  );
};

const CardMenu: Component<{ isOwnContent: boolean }> = (props) => {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger class="menu-trigger">
        <DotsThree size={20} />
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content class="menu-content">
          <DropdownMenu.Item class="menu-item">Copy Link</DropdownMenu.Item>
          <Show when={props.isOwnContent}>
            <DropdownMenu.Item class="menu-item">Edit</DropdownMenu.Item>
          </Show>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
};
