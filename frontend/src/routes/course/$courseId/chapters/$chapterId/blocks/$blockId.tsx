import { createFileRoute, useNavigate, Link } from "@tanstack/solid-router";
import { createQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal, createMemo } from "solid-js";
import * as Accordion from "@kobalte/core/accordion";
import * as Tabs from "@kobalte/core/tabs";
import type { Component, JSX } from "solid-js";
import { z } from "zod";
import { MarkdownEditor } from "~/components/shared/MarkdownEditor";
import { formatLastActivity } from "~/lib/utils";
import {
  Announcement,
  ContentBlock,
  CourseChapter,
  Course,
  type Note,
} from "~/lib/types";
import {
  Clock,
  Globe,
  CaretDown,
  CaretUp,
  Video,
  Article,
  PenNib,
  Barricade,
  PlayCircle,
  PlusCircle,
  CaretLeft,
  CaretRight,
} from "~/components/icons/Phosphor";

const fetchCourseContent = async (courseId: string): Promise<Course> => {
  console.log(`Fetching all content for course ID: ${courseId}`);
  await new Promise((r) => setTimeout(r, 500));
  return {
    id: parseInt(courseId),
    title: "Advanced Glazing Techniques",
    instructorName: "Master Wu",
    lastUpdatedAt: "2025-08-15T00:00:00Z",
    language: "English",
    thumbnailUrl: "",
    description: "",
    chapters: [
      {
        id: 1,
        title: "Introduction to Glaze Chemistry",
        contentBlocks: [
          {
            id: 101,
            type: "video",
            title: "Understanding Silica and Alumina",
            duration: "08:15",
          },
          { id: 102, type: "reading", title: "The Role of Fluxes" },
        ],
      },
      {
        id: 2,
        title: "Application Techniques",
        contentBlocks: [
          {
            id: 201,
            type: "video",
            title: "Dipping vs. Spraying",
            duration: "12:30",
          },
          { id: 202, type: "assignment", title: "Practice Plate" },
        ],
      },
    ],
    announcements: [
      {
        id: 1,
        authorNickname: "Master Wu",
        authorAvatarUrl: "https://placehold.co/40x40/d1fae5/065f46?text=W",
        createdAt: "2025-08-19T10:00:00Z",
        content:
          "Welcome to the course! Please review the syllabus in Chapter 1.",
      },
    ],
    userNotes: [
      {
        id: 1,
        title: "Flux temperature question",
        content: "Need to ask about eutectic points for feldspar.",
        createdAt: "2025-08-22T14:00:00Z",
      },
    ],
  };
};

// --- Route Definition ---

const courseContentSearchSchema = z.object({
  tab: z
    .enum(["overview", "announcements", "notes"])
    .default("overview")
    .catch("overview"),
});

type CourseTab = z.infer<typeof courseContentSearchSchema>["tab"];

export const Route = createFileRoute(
  "/course/$courseId/chapters/$chapterId/blocks/$blockId",
)({
  validateSearch: (search) => courseContentSearchSchema.parse(search),
  loader: ({ params: { courseId } }) => fetchCourseContent(courseId),
  component: CourseContentPage,
});

// --- Main Page Component ---

function CourseContentPage() {
  const params = Route.useParams();
  const search = Route.useSearch();
  const initialData = Route.useLoaderData();
  const courseQuery = createQuery(() => ({
    queryKey: ["course-content", params().courseId],
    queryFn: () => fetchCourseContent(params().courseId),
    initialData: initialData,
  }));

  const allBlocks = createMemo(
    () =>
      courseQuery.data?.chapters.flatMap((c: CourseChapter) =>
        c.contentBlocks.map((b: ContentBlock) => ({ ...b, chapterId: c.id })),
      ) ?? [],
  );

  const activeBlock = createMemo(() => {
    const blockId = parseInt(params().blockId);
    return allBlocks().find((b: ContentBlock) => b.id === blockId) ?? null;
  });

  return (
    <div class="course-content-layout">
      <main class="main-view">
        <Suspense
          fallback={<div class="video-placeholder">Loading Content...</div>}
        >
          <Show
            when={activeBlock()}
            fallback={
              <div class="video-placeholder">Select a lesson to begin.</div>
            }
          >
            {(block) => (
              <ContentViewer
                block={block()}
                allBlocks={allBlocks()}
                courseId={params().courseId}
              />
            )}
          </Show>
          <Show when={courseQuery.data}>
            {(course) => <InfoTabs course={course()} />}
          </Show>
        </Suspense>
      </main>
      <aside class="sidebar-outline">
        <Suspense fallback={<p>Loading outline...</p>}>
          <Show when={courseQuery.data}>
            {(course) => (
              <CourseOutline
                course={course()}
                activeBlockId={activeBlock()?.id}
              />
            )}
          </Show>
        </Suspense>
      </aside>
    </div>
  );
}

// --- Sub-Components ---

const ContentViewer: Component<{
  block: ContentBlock & { chapterId: number };
  allBlocks: (ContentBlock & { chapterId: number })[];
  courseId: string;
}> = (props) => {
  const navigate = useNavigate({ from: Route.fullPath });

  const currentIndex = createMemo(() =>
    props.allBlocks.findIndex((b) => b.id === props.block.id),
  );

  const navigateToBlock = (index: number) => {
    const targetBlock = props.allBlocks[index];
    if (targetBlock) {
      navigate({
        to: "/course/$courseId/chapters/$chapterId/blocks/$blockId",
        params: {
          courseId: props.courseId,
          chapterId: targetBlock.chapterId.toString(),
          blockId: targetBlock.id.toString(),
        },
        search: (prev) => prev, // Preserves current search params (like the active tab)
      });
    }
  };

  return (
    <div class="video-placeholder">
      <button
        class="carousel-btn left-0"
        onClick={() => navigateToBlock(currentIndex() - 1)}
        disabled={currentIndex() === 0}
      >
        <CaretLeft />
      </button>
      <p class="text-2xl">{props.block.title}</p>
      <button
        class="carousel-btn right-0"
        onClick={() => navigateToBlock(currentIndex() + 1)}
        disabled={currentIndex() === props.allBlocks.length - 1}
      >
        <CaretRight />
      </button>
    </div>
  );
};

const InfoTabs: Component<{ course: Course }> = (props) => {
  const search = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  return (
    <Tabs.Root
      class="info-tabs"
      value={search().tab}
      onChange={(tab) =>
        navigate({ search: (prev) => ({ ...prev, tab: tab as CourseTab }) })
      }
    >
      <Tabs.List class="info-tabs-list">
        <Tabs.Trigger value="overview">Overview</Tabs.Trigger>
        <Tabs.Trigger value="announcements">Announcements</Tabs.Trigger>
        <Tabs.Trigger value="notes">Notes</Tabs.Trigger>
      </Tabs.List>
      <div class="info-tabs-content">
        <Tabs.Content value="overview">
          <OverviewTab course={props.course} />
        </Tabs.Content>
        <Tabs.Content value="announcements">
          <AnnouncementsTab announcements={props.course.announcements || []} />
        </Tabs.Content>
        <Tabs.Content value="notes">
          <NotesTab notes={props.course.userNotes || []} />
        </Tabs.Content>
      </div>
    </Tabs.Root>
  );
};

const OverviewTab: Component<{ course: Course }> = (props) => {
  return (
    <div class="space-y-4">
      <h2 class="text-2xl font-bold">{props.course.title}</h2>
      <p>Created by {props.course.instructorName}</p>
      <div class="flex items-center gap-4 text-sm text-gray-600">
        <span class="flex items-center gap-1.5">
          <Clock /> Last updated{" "}
          {formatLastActivity(props.course.lastUpdatedAt)}
        </span>
        <span class="flex items-center gap-1.5">
          <Globe /> {props.course.language}
        </span>
      </div>
    </div>
  );
};
const AnnouncementsTab: Component<{ announcements: Announcement[] }> = (
  props,
) => {
  return (
    <div class="space-y-6">
      <For each={props.announcements} fallback={<p>No announcements yet.</p>}>
        {(announcement) => (
          <div class="flex gap-4">
            <img
              src={announcement.authorAvatarUrl}
              alt=""
              class="w-10 h-10 rounded-full"
            />
            <div>
              <p class="font-semibold">
                {announcement.authorNickname}{" "}
                <span class="font-normal text-gray-500">
                  posted an announcement ·{" "}
                  {formatLastActivity(announcement.createdAt)}
                </span>
              </p>
              <p class="mt-1">{announcement.content}</p>
            </div>
          </div>
        )}
      </For>
    </div>
  );
};
const NotesTab: Component<{ notes: Note[] }> = (props) => {
  const [isEditing, setIsEditing] = createSignal(false);
  return (
    <div class="space-y-4">
      <Show
        when={isEditing()}
        fallback={
          <button class="new-note-trigger" onClick={() => setIsEditing(true)}>
            <span>Create a new note at 02:34</span>
            <PlusCircle size={20} />
          </button>
        }
      >
        <MarkdownEditor
          onCancel={() => setIsEditing(false)}
          onSubmit={() => setIsEditing(false)}
        />
      </Show>
      <Accordion.Root class="notes-accordion" multiple>
        <For
          each={props.notes}
          fallback={<p>You haven't taken any notes for this chapter yet.</p>}
        >
          {(note) => (
            <Accordion.Item value={note.id.toString()} class="note-item">
              <Accordion.Header>
                <Accordion.Trigger class="note-trigger">
                  {note.title}
                  <CaretDown class="note-caret" />
                </Accordion.Trigger>
              </Accordion.Header>
              <Accordion.Content class="note-content">
                {note.content}
              </Accordion.Content>
            </Accordion.Item>
          )}
        </For>
      </Accordion.Root>
    </div>
  );
};

const CourseOutline: Component<{
  course: Course;
  activeBlockId?: number;
}> = (props) => {
  const search = Route.useSearch();
  const contentIcons = {
    video: Video,
    reading: Article,
    assignment: PenNib,
    quiz: Barricade,
  };
  return (
    <div class="h-full overflow-y-auto">
      <h2 class="p-4 font-bold text-lg border-b">Course content</h2>
      <Accordion.Root class="outline-accordion" multiple defaultValue={["1"]}>
        <For each={props.course.chapters}>
          {(chapter) => (
            <Accordion.Item
              value={chapter.id.toString()}
              class="outline-chapter"
            >
              <Accordion.Header>
                <Accordion.Trigger class="outline-chapter-trigger">
                  <span class="font-semibold">{chapter.title}</span>
                  <CaretDown class="outline-caret" />
                </Accordion.Trigger>
              </Accordion.Header>
              <Accordion.Content class="outline-chapter-content">
                <ul class="list-none p-0 m-0">
                  <For each={chapter.contentBlocks}>
                    {(block) => {
                      const Icon = contentIcons[block.type];
                      return (
                        <li>
                          <Link
                            to="/course/$courseId/chapters/$chapterId/blocks/$blockId"
                            params={{
                              courseId: props.course.id.toString(),
                              chapterId: chapter.id.toString(),
                              blockId: block.id.toString(),
                            }}
                            search={search()}
                            class="outline-block-item"
                            data-active={props.activeBlockId === block.id}
                          >
                            <Icon size={18} class="flex-shrink-0" />
                            <span class="flex-grow truncate">
                              {block.title}
                            </span>
                            <Show when={block.duration}>
                              <span class="text-xs text-gray-500">
                                {block.duration}
                              </span>
                            </Show>
                          </Link>
                        </li>
                      );
                    }}
                  </For>
                </ul>
              </Accordion.Content>
            </Accordion.Item>
          )}
        </For>
      </Accordion.Root>
    </div>
  );
};
