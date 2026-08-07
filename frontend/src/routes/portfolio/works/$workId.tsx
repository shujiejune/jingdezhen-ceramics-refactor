import { createFileRoute, Link } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal, createEffect, on } from "solid-js";
import type { Component } from "solid-js";
import { marked } from "marked";
import * as Dialog from "@kobalte/core/dialog";
import {
  ArrowFatUp,
  BookmarkSimple,
  ShareFat,
  NotePencil,
  Trash,
} from "~/components/icons/Phosphor";
import { PortfolioWork, PortfolioWorkImage } from "~/lib/types";

// --- Mock User Data ---
const mockUser = {
  isLoggedIn: true,
  id: "user-1", // This user is the owner of the work
};

// --- Mock API Fetching ---

const fetchWorkById = async (workId: string): Promise<PortfolioWork> => {
  console.log(`Fetching work by ID: ${workId}`);
  await new Promise((r) => setTimeout(r, 500));

  // In a real app, you would fetch this from your backend.
  // Here, we just return a detailed mock object.
  return {
    id: parseInt(workId),
    userId: "user-1",
    creatorNickname: "Artist A",
    creatorAvatarUrl: "https://placehold.co/40x40/e0e7ff/4338ca?text=A",
    title: `Ceramic Vessel No. ${workId}`,
    description: `
This is a *detailed description* of the **Ceramic Vessel No. ${workId}**.

- Hand-thrown on the potter's wheel.
- Fired to cone 10 in a reduction atmosphere.
- Features a beautiful celadon glaze with copper red highlights.

The process was challenging but ultimately rewarding, pushing the boundaries of my usual techniques.
    `,
    isEditorsChoice: false,
    upvotesCount: 188,
    createdAt: "2025-09-04T10:00:00Z",
    updatedAt: "2025-09-05T14:30:00Z",
    thumbnailUrl: "https://placehold.co/800x600/d1fae5/065f46?text=Thumbnail",
    images: [
      {
        id: 1,
        imageUrl: "https://placehold.co/800x600/d1fae5/065f46?text=Detail+1",
        isThumbnail: true,
        caption: "Main view of the vessel.",
        displayOrder: 1,
      },
      {
        id: 2,
        imageUrl: "https://placehold.co/800x600/e0f2fe/0891b2?text=Detail+2",
        isThumbnail: false,
        caption: "A close-up of the glaze crazing.",
        displayOrder: 2,
      },
      {
        id: 3,
        imageUrl: "https://placehold.co/800x600/f3e8ff/7e22ce?text=Detail+3",
        isThumbnail: false,
        caption: "The foot of the pot, showing the maker's mark.",
        displayOrder: 3,
      },
    ],
    tags: [
      { id: 1, name: "Glazing" },
      { id: 2, name: "Stoneware" },
    ],
    upvotedByMe: true,
    savedByMe: false,
  };
};

// --- Route Definition ---
export const Route = createFileRoute("/portfolio/works/$workId")({
  component: SingleWorkPage,
});

// --- Main Page Component ---
function SingleWorkPage() {
  const params = Route.useParams();
  const workQuery = useQuery(() => ({
    queryKey: ["portfolio-work", params().workId],
    queryFn: () => fetchWorkById(params().workId),
  }));

  return (
    <div class="container mx-auto px-4 py-8">
      <header class="text-center mb-8">
        <h1 class="text-4xl font-bold">Portfolio</h1>
        <p class="text-lg text-gray-600 mt-2">
          Discover the exceptional talent of our community.
        </p>
      </header>

      <Suspense fallback={<WorkDetailSkeleton />}>
        <Show when={workQuery.data} keyed>
          {(work) => <WorkDetail work={work} />}
        </Show>
      </Suspense>
    </div>
  );
}

// --- Sub-Components ---

const WorkDetail: Component<{ work: PortfolioWork }> = (props) => {
  const [isEditing, setIsEditing] = createSignal(false);
  const isOwner = () =>
    mockUser.isLoggedIn && mockUser.id === props.work.userId;

  const formatDate = (isoString: string) => {
    return new Date(isoString).toLocaleDateString("en-US", {
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  };

  const [parsedDescription, setParsedDescription] = createSignal("");
  createEffect(
    on(
      () => props.work.description,
      async (desc) => {
        if (desc) {
          try {
            const html = await marked.parse(desc);
            setParsedDescription(html);
          } catch (e) {
            setParsedDescription(desc); // Fallback to raw text on error
          }
        } else {
          setParsedDescription("");
        }
      },
    ),
  );

  return (
    <article>
      <h2 class="text-3xl font-bold text-center mb-4">{props.work.title}</h2>

      <div class="flex justify-center items-center gap-4 text-sm text-gray-600 mb-8">
        <div class="flex items-center gap-2">
          <img
            src={props.work.creatorAvatarUrl}
            alt={props.work.creatorNickname}
            class="w-8 h-8 rounded-full"
          />
          <span>{props.work.creatorNickname}</span>
        </div>
        <span> · </span>
        <span>Created on {formatDate(props.work.createdAt)}</span>
        <span> · </span>
        <span>Updated on {formatDate(props.work.updatedAt)}</span>
        <Show when={isOwner()}>
          <div class="flex items-center gap-3 ml-4">
            <button
              onClick={() => setIsEditing(!isEditing())}
              class="text-gray-500 hover:text-blue-600"
            >
              <NotePencil size={20} />
            </button>
            <DeleteWorkDialog workId={props.work.id} />
          </div>
        </Show>
      </div>

      <div class="flex flex-col md:flex-row gap-8 lg:gap-12">
        {/* Left Column: Images */}
        <div class="w-full md:w-2/3 lg:w-3/4">
          <Show
            when={!isEditing()}
            fallback={<EditModeImages images={props.work.images || []} />}
          >
            <div class="space-y-6">
              <For
                each={[...(props.work.images || [])].sort(
                  (a, b) => a.displayOrder - b.displayOrder,
                )}
              >
                {(image) => (
                  <figure>
                    <img
                      src={image.imageUrl}
                      alt={image.caption}
                      class="w-full rounded-lg shadow-md"
                    />
                    <Show when={image.caption}>
                      <figcaption class="text-center text-sm text-gray-600 mt-2">
                        {image.caption}
                      </figcaption>
                    </Show>
                  </figure>
                )}
              </For>
            </div>
          </Show>
        </div>

        {/* Right Column: Details */}
        <aside class="w-full md:w-1/3 lg:w-1/4 md:sticky top-8 self-start">
          <div class="space-y-6 p-6 border rounded-lg bg-gray-50">
            <div>
              <button
                class="w-full flex items-center justify-center gap-2 py-3 border border-red-500 rounded-lg text-lg font-semibold"
                classList={{
                  "bg-red-500 text-white": props.work.upvotedByMe,
                  "text-red-500 bg-white hover:bg-red-50":
                    !props.work.upvotedByMe,
                }}
              >
                <ArrowFatUp fill="currentColor" />
                <span>{props.work.upvotesCount} Upvotes</span>
              </button>
            </div>

            <div>
              <h3 class="font-semibold mb-2">Description</h3>
              <div
                class="prose prose-sm max-w-none"
                innerHTML={parsedDescription()}
              />
            </div>

            <div>
              <h3 class="font-semibold mb-2">Tags</h3>
              <div class="flex flex-wrap gap-2">
                <For each={props.work.tags}>
                  {(tag) => <span class="tag-capsule">{tag.name}</span>}
                </For>
              </div>
            </div>

            <div class="flex items-center justify-around border-t pt-4">
              <button
                class="flex items-center gap-2 hover:text-blue-600"
                classList={{ "text-blue-600": props.work.savedByMe }}
              >
                <BookmarkSimple
                  fill={props.work.savedByMe ? "currentColor" : "none"}
                  size={24}
                />
                <span>Save</span>
              </button>
              <button class="flex items-center gap-2 hover:text-green-600">
                <ShareFat size={24} />
                <span>Share</span>
              </button>
            </div>
          </div>
        </aside>
      </div>
    </article>
  );
};

const EditModeImages: Component<{ images: PortfolioWorkImage[] }> = (props) => {
  return (
    <div class="border-2 border-dashed rounded-lg p-6 text-center bg-gray-50">
      <p class="mb-4 text-gray-600">
        Drag & drop to reorder images, or upload new ones.
      </p>
      {/* Implement drag-and-drop and file upload logic here */}
      <div class="grid grid-cols-3 gap-4">
        <For each={props.images}>
          {(image) => (
            <div class="relative group">
              <img
                src={image.imageUrl}
                class="w-full h-32 object-cover rounded"
              />
              <div class="absolute inset-0 bg-black/50 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
                <button class="text-white">
                  <Trash size={24} />
                </button>
              </div>
            </div>
          )}
        </For>
        <div class="w-full h-32 border-2 border-dashed rounded flex items-center justify-center text-gray-400 hover:bg-gray-100 cursor-pointer">
          Add Image
        </div>
      </div>
    </div>
  );
};

const DeleteWorkDialog: Component<{ workId: number }> = (props) => {
  const [isOpen, setIsOpen] = createSignal(false);

  const handleDelete = () => {
    console.log(`Deleting work ${props.workId}...`);
    // Add deletion logic here
    setIsOpen(false);
  };

  return (
    <Dialog.Root open={isOpen()} onOpenChange={setIsOpen}>
      <Dialog.Trigger class="text-gray-500 hover:text-red-600">
        <Trash size={20} />
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay class="dialog-overlay" />
        <div class="dialog-positioner">
          <Dialog.Content class="dialog-content">
            <Dialog.Title class="dialog-title">Confirm Deletion</Dialog.Title>
            <Dialog.Description class="dialog-description">
              Are you sure you want to delete this portfolio work? This action
              cannot be undone.
            </Dialog.Description>
            <div class="flex justify-end gap-4 mt-6">
              <Dialog.CloseButton class="btn-secondary">
                Cancel
              </Dialog.CloseButton>
              <button onClick={handleDelete} class="btn-danger">
                Delete
              </button>
            </div>
          </Dialog.Content>
        </div>
      </Dialog.Portal>
    </Dialog.Root>
  );
};

const WorkDetailSkeleton: Component = () => (
  <div>
    <div class="h-8 w-3/4 mx-auto bg-gray-200 rounded-md animate-pulse mb-4" />
    <div class="h-6 w-1/2 mx-auto bg-gray-200 rounded-md animate-pulse mb-8" />
    <div class="flex flex-col md:flex-row gap-12">
      <div class="w-full md:w-3/4 space-y-6">
        <div class="w-full h-96 bg-gray-200 rounded-lg animate-pulse" />
        <div class="w-full h-96 bg-gray-200 rounded-lg animate-pulse" />
      </div>
      <div class="w-full md:w-1/4">
        <div class="h-80 bg-gray-200 rounded-lg animate-pulse" />
      </div>
    </div>
  </div>
);
