import { createFileRoute } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal, createEffect } from "solid-js";
import type { Component, JSX } from "solid-js";
import { marked } from "marked";
import * as Accordion from "@kobalte/core/accordion";
import * as Tooltip from "@kobalte/core/tooltip";
import { Artwork, ArtworkImage, Note } from "~/lib/types";
import { MarkdownEditor } from "~/components/shared/MarkdownEditor";
import {
  Heart,
  ShareFat,
  NotePencil,
  PencilSimpleLine,
  CaretDown,
} from "~/components/icons/Phosphor";

// --- Mock User Data ---
const mockUser = {
  isLoggedIn: true,
  // isLoggedIn: false, // <-- Toggle this to test guest view
};

// --- Mock API Fetching ---
const fetchArtworkDetails = async (artworkId: string): Promise<Artwork> => {
  console.log(`Fetching details for artwork ID: ${artworkId}`);
  await new Promise((r) => setTimeout(r, 400));
  return {
    id: parseInt(artworkId),
    title: "Blue-and-white Plate with Dragon Design",
    artistName: "Unknown Imperial Kiln Artist",
    period: "Ming Dynasty, Yongle Period (1403-1424)",
    dimensions: "Height: 7.5 cm, Diameter: 45.2 cm",
    description: `
This large plate is a quintessential example of early Ming dynasty blue-and-white porcelain. The central medallion features a powerful, three-clawed dragon writhing amidst stylized clouds and flames. The cobalt blue is rich and deep, showing the characteristic 'heaping and piling' effect of the imported cobalt of the era.

The cavetto is decorated with a continuous scroll of peony, and the rim features a classic wave pattern. The base is unglazed, revealing the fine, white porcelain body.
    `,
    thumbnailUrl: "https://placehold.co/800x800/e0f2fe/0891b2?text=Artwork+1",
    images: [
      {
        id: 1,
        artworkId: 1,
        imageUrl:
          "https://placehold.co/800x800/e0f2fe/0891b2?text=View+1+(Thumbnail)",
        isPrimary: true,
        caption: "Main view",
        displayOrder: 1,
      },
      {
        id: 2,
        artworkId: 1,
        imageUrl:
          "https://placehold.co/800x800/dbeafe/1e40af?text=View+2+(Dragon+Detail)",
        isPrimary: false,
        caption: "Dragon Detail",
        displayOrder: 2,
      },
      {
        id: 3,
        artworkId: 1,
        imageUrl:
          "https://placehold.co/800x800/bfdbfe/1d4ed8?text=View+3+(Base)",
        isPrimary: false,
        caption: "Base view",
        displayOrder: 3,
      },
      {
        id: 4,
        artworkId: 1,
        imageUrl:
          "https://placehold.co/800x800/93c5fd/2563eb?text=View+4+(Rim+Detail)",
        isPrimary: false,
        caption: "Rim Detail",
        displayOrder: 4,
      },
    ],
    tags: [
      { id: 1, name: "Blue-and-white" },
      { id: 2, name: "Ming Dynasty" },
      { id: 3, name: "Imperial Ware" },
    ],
    category: "Blue-and-white",
    isFavorite: true,
    favoriteCount: 128,
    noteCount: 2,
    createdAt: "2025-01-15T09:00:00Z",
  };
};

const fetchUserNotesForArtwork = async (artworkId: string): Promise<Note[]> => {
  if (!mockUser.isLoggedIn) return [];
  console.log(`Fetching notes for artwork ID: ${artworkId}`);
  await new Promise((r) => setTimeout(r, 600));
  return [
    {
      id: 1,
      title: "Cobalt Blue observations",
      content:
        "The 'heaping and piling' is very pronounced here. Need to compare with Xuande period examples.",
      createdAt: "2025-08-10T11:00:00Z",
      updatedAt: "2025-08-12T15:45:00Z",
    },
    {
      id: 2,
      title: "Dragon Motif study",
      content:
        "The three-clawed dragon is typical for this period, often associated with princes or high-ranking nobility rather than the emperor.",
      createdAt: "2025-09-01T18:20:00Z",
      updatedAt: "2025-09-01T18:20:00Z",
    },
  ];
};

// --- Route Definition ---
export const Route = createFileRoute("/gallery/artworks/$artworkId")({
  component: ArtworkDetailPage,
});

// --- Main Page Component ---
function ArtworkDetailPage() {
  const params = Route.useParams();

  const artworkQuery = useQuery(() => ({
    queryKey: ["artwork", params().artworkId],
    queryFn: () => fetchArtworkDetails(params().artworkId),
  }));

  const notesQuery = useQuery(() => ({
    queryKey: ["artwork-notes", params().artworkId],
    queryFn: () => fetchUserNotesForArtwork(params().artworkId),
    get enabled() {
      return mockUser.isLoggedIn;
    },
  }));

  return (
    <div class="container mx-auto px-4 py-8">
      <header class="text-center mb-12">
        <h1 class="text-4xl font-bold">Gallery</h1>
        <p class="text-lg text-gray-600 mt-2">
          Explore timeless masterpieces from renowned collections.
        </p>
      </header>

      <Suspense fallback={<ArtworkDetailSkeleton />}>
        <Show when={artworkQuery.data} keyed>
          {(artwork) => (
            <div class="grid grid-cols-1 lg:grid-cols-12 gap-8">
              <ImageGallery images={artwork.images || []} />
              <ArtworkInfo artwork={artwork} notes={notesQuery.data || []} />
            </div>
          )}
        </Show>
      </Suspense>
    </div>
  );
}

// --- Sub-Components ---

const ImageGallery: Component<{ images: ArtworkImage[] }> = (props) => {
  const [selectedImage, setSelectedImage] = createSignal(
    props.images.find((img) => img.isPrimary) || props.images[0],
  );

  return (
    <>
      {/* Left Column: Thumbnails */}
      <aside class="hidden lg:block lg:col-span-2 sticky top-8 self-start">
        <div class="space-y-3 max-h-[80vh] overflow-y-auto pr-2">
          <For each={props.images}>
            {(image) => (
              <button
                onClick={() => setSelectedImage(image)}
                class="w-full h-24 rounded-md overflow-hidden border-2 transition"
                classList={{
                  "border-blue-500": selectedImage()?.id === image.id,
                  "border-transparent hover:border-gray-300":
                    selectedImage()?.id !== image.id,
                }}
              >
                <img
                  src={image.imageUrl}
                  alt={image.caption}
                  class="w-full h-full object-cover"
                />
              </button>
            )}
          </For>
        </div>
      </aside>

      {/* Middle Column: Main Display */}
      <div class="col-span-12 lg:col-span-6 sticky top-8 self-start">
        <div class="w-full aspect-square bg-gray-100 rounded-lg overflow-hidden">
          <Show
            when={selectedImage()}
            fallback={<div class="w-full h-full bg-gray-200 animate-pulse" />}
          >
            <img
              src={selectedImage()!.imageUrl}
              alt={selectedImage()!.caption}
              class="w-full h-full object-contain"
            />
          </Show>
        </div>
      </div>
    </>
  );
};

const ArtworkInfo: Component<{ artwork: Artwork; notes: Note[] }> = (props) => {
  const [showNoteEditor, setShowNoteEditor] = createSignal(false);
  const [isCopied, setIsCopied] = createSignal(false);

  const copyLink = async () => {
    const link = `${window.location.origin}/gallery/artworks/${props.artwork.id}`;
    try {
      await navigator.clipboard.writeText(link);
      setIsCopied(true);
      setTimeout(() => setIsCopied(false), 2000); // Hide tooltip after 2 seconds
    } catch (err) {
      console.error("Failed to copy link:", err);
    }
  };

  return (
    <main class="col-span-12 lg:col-span-4">
      <div class="space-y-4">
        <h2 class="text-3xl font-bold">{props.artwork.title}</h2>
        <div class="text-gray-600 space-y-1">
          <p>
            <span class="font-semibold text-gray-800">Artist:</span>{" "}
            {props.artwork.artistName}
          </p>
          <p>
            <span class="font-semibold text-gray-800">Period:</span>{" "}
            {props.artwork.period}
          </p>
          <p>
            <span class="font-semibold text-gray-800">Dimensions:</span>{" "}
            {props.artwork.dimensions}
          </p>
        </div>
        <div>
          <h3 class="font-semibold text-gray-800 mb-1">Description</h3>
          <div class="prose prose-sm max-w-none text-gray-700">
            {props.artwork.description}
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <For each={props.artwork.tags}>
            {(tag) => <span class="tag-capsule">{tag.name}</span>}
          </For>
        </div>
      </div>

      <div class="flex items-center gap-4 my-6 border-y py-4">
        <ActionButton
          userAction={true}
          label="Favorite"
          count={props.artwork.favoriteCount}
        >
          <Heart
            size={20}
            fill={props.artwork.isFavorite ? "currentColor" : "none"}
            classList={{ "text-red-500": props.artwork.isFavorite }}
          />
        </ActionButton>
        <Tooltip.Root open={isCopied()} onOpenChange={setIsCopied}>
          <Tooltip.Trigger as="div">
            <ActionButton label="Share" onClick={copyLink}>
              <ShareFat size={20} />
            </ActionButton>
          </Tooltip.Trigger>
          <Tooltip.Portal>
            <Tooltip.Content class="tooltip-content">
              Link Copied!
            </Tooltip.Content>
          </Tooltip.Portal>
        </Tooltip.Root>
        <ActionButton
          userAction={true}
          label="Notes"
          onClick={() => setShowNoteEditor(true)}
        >
          <NotePencil size={20} />
        </ActionButton>
      </div>

      <Show when={showNoteEditor()}>
        <div class="mb-6">
          <h3 class="text-xl font-semibold mb-3">Create a New Note</h3>
          <MarkdownEditor
            onCancel={() => setShowNoteEditor(false)}
            onSubmit={() => {
              console.log("Note Submitted");
              setShowNoteEditor(false);
            }}
          />
        </div>
      </Show>

      <Show when={mockUser.isLoggedIn && props.notes.length > 0}>
        <div class="space-y-4">
          <h3 class="text-xl font-semibold">Your Notes</h3>
          <UserNotes notes={props.notes} />
        </div>
      </Show>
    </main>
  );
};

const ActionButton: Component<{
  userAction?: boolean;
  label: string;
  count?: number;
  onClick?: () => void;
  children: JSX.Element;
}> = (props) => {
  const handleClick = () => {
    if (props.onClick) {
      props.onClick();
    }
  };

  const buttonContent = (
    <button
      onClick={handleClick}
      class="flex items-center gap-2 text-gray-700 hover:text-blue-600 disabled:hover:text-gray-700 disabled:opacity-50"
    >
      {props.children}
      <span class="font-semibold">{props.label}</span>
      <Show when={props.count !== undefined}>
        <span class="text-sm text-gray-500">{props.count}</span>
      </Show>
    </button>
  );

  return (
    <Show when={props.userAction && !mockUser.isLoggedIn}>
      <Tooltip.Root>
        <Tooltip.Trigger as="div">{buttonContent}</Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Content class="tooltip-content max-w-xs text-center">
            Please log in or register to use this feature.
          </Tooltip.Content>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Show>
  );
};

const MarkdownRenderer: Component<{ content: string }> = (props) => {
  const [html, setHtml] = createSignal("");

  createEffect(async () => {
    if (props.content) {
      const parsedHtml = await marked.parse(props.content);
      setHtml(parsedHtml);
    } else {
      setHtml("");
    }
  });

  return <div class="prose prose-sm max-w-none" innerHTML={html()} />;
};

const UserNotes: Component<{ notes: Note[] }> = (props) => {
  const formatDate = (iso: string) => new Date(iso).toLocaleDateString("en-CA");

  return (
    <Accordion.Root class="w-full" multiple>
      <For each={props.notes}>
        {(note) => (
          <Accordion.Item value={note.id.toString()} class="border-b">
            <Accordion.Header class="w-full">
              <Accordion.Trigger class="flex justify-between items-center w-full py-3 text-left hover:bg-gray-50 group">
                <div>
                  <p class="font-semibold">{note.title}</p>
                  <p class="text-xs text-gray-500 mt-1">
                    Created: {formatDate(note.createdAt)} | Updated:{" "}
                    {formatDate(note.updatedAt)}
                  </p>
                </div>
                <div class="flex items-center gap-4">
                  <button
                    class="text-gray-500 hover:text-blue-600"
                    onClick={(e) => {
                      e.stopPropagation();
                      console.log("Edit note");
                    }}
                  >
                    <PencilSimpleLine />
                  </button>
                  <CaretDown class="transition-transform duration-200 group-data-[expanded]:rotate-180" />
                </div>
              </Accordion.Trigger>
            </Accordion.Header>
            <Accordion.Content class="p-4 bg-gray-50 text-sm">
              <MarkdownRenderer content={note.content} />
            </Accordion.Content>
          </Accordion.Item>
        )}
      </For>
    </Accordion.Root>
  );
};

const ArtworkDetailSkeleton: Component = () => (
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-8 animate-pulse">
    <div class="hidden lg:block lg:col-span-2 space-y-3">
      <div class="w-full h-24 bg-gray-200 rounded-md"></div>
      <div class="w-full h-24 bg-gray-200 rounded-md"></div>
      <div class="w-full h-24 bg-gray-200 rounded-md"></div>
    </div>
    <div class="col-span-12 lg:col-span-6">
      <div class="w-full aspect-square bg-gray-200 rounded-lg"></div>
    </div>
    <div class="col-span-12 lg:col-span-4 space-y-6">
      <div class="h-8 w-3/4 bg-gray-200 rounded"></div>
      <div class="space-y-2">
        <div class="h-4 w-1/2 bg-gray-200 rounded"></div>
        <div class="h-4 w-2/3 bg-gray-200 rounded"></div>
        <div class="h-4 w-1/2 bg-gray-200 rounded"></div>
      </div>
      <div class="h-20 w-full bg-gray-200 rounded"></div>
    </div>
  </div>
);
