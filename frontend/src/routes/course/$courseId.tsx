import { createFileRoute } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal } from "solid-js";
import * as Accordion from "@kobalte/core/accordion";
import * as Dialog from "@kobalte/core/dialog";
import type { Component } from "solid-js";
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
  X,
} from "~/components/icons/Phosphor";
import { formatLastActivity } from "~/lib/utils";
import { CourseChapter, Course } from "~/lib/types";

// Mock function to fetch course details
const fetchCourseDetails = async (courseId: string): Promise<Course> => {
  console.log(`Fetching details for course ID: ${courseId}`);
  await new Promise((r) => setTimeout(r, 500));
  return {
    id: parseInt(courseId),
    title: "Advanced Glazing Techniques",
    instructorName: "Master Wu",
    lastUpdatedAt: "2025-08-15T00:00:00Z",
    language: "English",
    thumbnailUrl: "https://placehold.co/600x400/d1fae5/065f46?text=Glazing",
    description:
      "This course delves into the intricate world of ceramic glazing, moving beyond the basics to explore complex chemical interactions, application methods, and firing schedules. Students will learn to create stunning, unique surfaces by understanding the science behind the art. We will cover everything from crystalline and celadon glazes to advanced raku firing.",
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
            isPreviewable: true,
            videoUrl: "https://www.w3schools.com/html/mov_bbb.mp4",
          },
          { id: 102, type: "reading", title: "The Role of Fluxes" },
          { id: 103, type: "quiz", title: "Chemistry Basics Quiz" },
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
          {
            id: 202,
            type: "video",
            title: "Brushwork and Wax Resist",
            duration: "15:45",
          },
          { id: 203, type: "assignment", title: "Practice Plate" },
        ],
      },
    ],
  };
};

// --- Route Definition ---

export const Route = createFileRoute("/course/$courseId")({
  loader: ({ params: { courseId } }) => fetchCourseDetails(courseId),
  component: CourseDetailPage,
});

// --- Main Page Component ---

function CourseDetailPage() {
  const params = Route.useParams();
  const courseQuery = useQuery(() => ({
    queryKey: ["course-details", params().courseId],
    queryFn: () => fetchCourseDetails(params().courseId),
  }));

  return (
    <div class="bg-white min-h-full">
      <Suspense fallback={<div class="h-screen">Loading...</div>}>
        <Show when={courseQuery.data}>
          {(course) => (
            <>
              <CourseBanner course={course()} />
              <div class="course-body-layout">
                <div class="course-main-content">
                  <CourseDescription description={course().description} />
                  <CourseContent chapters={course().chapters} />
                </div>
                <FloatingEnrollPanel thumbnailUrl={course().thumbnailUrl} />
              </div>
            </>
          )}
        </Show>
      </Suspense>
    </div>
  );
}

// --- Sub-Components ---

const CourseBanner: Component<{ course: Course }> = (props) => (
  <header class="course-banner">
    <div class="banner-content">
      <h1 class="text-4xl font-bold">{props.course.title}</h1>
      <p class="mt-4 text-xl">Created by {props.course.instructorName}</p>
      <div class="mt-2 text-sm flex items-center gap-4">
        <span class="flex items-center gap-1.5">
          <Clock /> Last updated{" "}
          {formatLastActivity(props.course.lastUpdatedAt)}
        </span>
        <span class="flex items-center gap-1.5">
          <Globe /> {props.course.language}
        </span>
      </div>
    </div>
  </header>
);

const FloatingEnrollPanel: Component<{ thumbnailUrl: string }> = (props) => (
  <aside class="enroll-panel">
    <div class="enroll-panel-inner">
      <img
        src={props.thumbnailUrl}
        alt="Course thumbnail"
        class="w-full aspect-video object-cover rounded-t-lg"
      />
      <div class="p-6">
        <button class="enroll-button">Enroll</button>
      </div>
    </div>
  </aside>
);

const CourseDescription: Component<{ description: string }> = (props) => (
  <section class="course-section">
    <h2 class="section-title">Description</h2>
    <div class="prose">
      <p>{props.description}</p>
    </div>
  </section>
);

const CourseContent: Component<{ chapters: CourseChapter[] }> = (props) => {
  const [previewVideoUrl, setPreviewVideoUrl] = createSignal<string | null>(
    null,
  );

  return (
    <>
      <section class="course-section">
        <h2 class="section-title">Course content</h2>
        <Accordion.Root class="accordion" multiple>
          <For each={props.chapters}>
            {(chapter) => (
              <ChapterAccordion
                chapter={chapter}
                onPreview={setPreviewVideoUrl}
              />
            )}
          </For>
        </Accordion.Root>
      </section>
      <VideoPlayerModal
        isOpen={!!previewVideoUrl()}
        url={previewVideoUrl()}
        onClose={() => setPreviewVideoUrl(null)}
      />
    </>
  );
};

const ChapterAccordion: Component<{
  chapter: CourseChapter;
  onPreview: (url: string) => void;
}> = (props) => {
  const contentIcons = {
    video: Video,
    reading: Article,
    assignment: PenNib,
    quiz: Barricade,
  };

  return (
    <Accordion.Item class="accordion-item" value={props.chapter.id.toString()}>
      <Accordion.Header class="accordion-header">
        <Accordion.Trigger class="accordion-trigger">
          <div class="flex items-center gap-2">
            <div class="accordion-caret">
              <CaretDown class="caret-down" />
              <CaretUp class="caret-up" />
            </div>
            <span class="font-bold">{props.chapter.title}</span>
          </div>
          <span class="text-sm text-gray-600">
            {props.chapter.contentBlocks.length} lectures
          </span>
        </Accordion.Trigger>
      </Accordion.Header>
      <Accordion.Content class="accordion-content">
        <ul class="content-block-list">
          <For each={props.chapter.contentBlocks}>
            {(block) => {
              const Icon = contentIcons[block.type];
              return (
                <li class="content-block-item">
                  <div class="flex items-center gap-4">
                    <Icon size={20} class="text-gray-700" />
                    <span>{block.title}</span>
                  </div>
                  <div class="flex items-center gap-4 text-sm">
                    <Show when={block.isPreviewable && block.videoUrl}>
                      <button
                        class="preview-button"
                        onClick={() => props.onPreview(block.videoUrl!)}
                      >
                        <PlayCircle size={18} /> Preview
                      </button>
                    </Show>
                    <Show when={block.duration}>
                      <span class="text-gray-600">{block.duration}</span>
                    </Show>
                  </div>
                </li>
              );
            }}
          </For>
        </ul>
      </Accordion.Content>
    </Accordion.Item>
  );
};

const VideoPlayerModal: Component<{
  isOpen: boolean;
  url: string | null;
  onClose: () => void;
}> = (props) => {
  return (
    <Dialog.Root open={props.isOpen} onOpenChange={props.onClose}>
      <Dialog.Portal>
        <Dialog.Overlay class="dialog-overlay" />
        <div class="dialog-positioner">
          <Dialog.Content class="dialog-content">
            <Dialog.CloseButton class="dialog-close-button">
              <X />
            </Dialog.CloseButton>
            <Show when={props.url}>
              <video src={props.url!} controls autoplay class="w-full h-full" />
            </Show>
          </Dialog.Content>
        </div>
      </Dialog.Portal>
    </Dialog.Root>
  );
};
