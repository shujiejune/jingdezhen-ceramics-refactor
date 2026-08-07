import { createFileRoute } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense } from "solid-js";
import { Link } from "@tanstack/solid-router";
import type { Component } from "solid-js";

// --- Data Fetching and Types ---

interface Course {
  id: number;
  title: string;
  instructorName: string;
  thumbnailUrl: string;
  progressPercent?: number; // Optional: only for enrolled users
}

// Mock function to simulate fetching courses from your backend
const fetchCourses = async (): Promise<Course[]> => {
  console.log("Fetching all courses...");
  await new Promise((r) => setTimeout(r, 500)); // Simulate network delay

  // In a real app, this would be a fetch call to your API:
  // const response = await fetch(`/api/courses`);
  // return response.json();

  return [
    {
      id: 1,
      title: "Introduction to Porcelain",
      instructorName: "Dr. Evelyn Reed",
      thumbnailUrl: "https://placehold.co/600x400/dbeafe/1e40af?text=Porcelain",
      progressPercent: 75,
    },
    {
      id: 2,
      title: "Advanced Glazing Techniques",
      instructorName: "Master Wu",
      thumbnailUrl: "https://placehold.co/600x400/d1fae5/065f46?text=Glazing",
      progressPercent: 30,
    },
    {
      id: 3,
      title: "History of Ming Ceramics",
      instructorName: "L. Chen",
      thumbnailUrl: "https://placehold.co/600x400/fef2f2/991b1b?text=History",
    },
    {
      id: 4,
      title: "The Art of the Tea Bowl",
      instructorName: "Yuki Tanaka",
      thumbnailUrl: "https://placehold.co/600x400/f0fdf4/166534?text=Tea+Bowl",
      progressPercent: 100,
    },
    {
      id: 5,
      title: "Sculptural Ceramics: Form & Expression",
      instructorName: "D. Miller",
      thumbnailUrl: "https://placehold.co/600x400/eff6ff/3730a3?text=Sculpture",
    },
    {
      id: 6,
      title: "Kiln Building Workshop",
      instructorName: "ClayMaster",
      thumbnailUrl: "https://placehold.co/600x400/fefce8/854d0e?text=Kiln",
    },
  ];
};

// --- Route Definition ---

export const Route = createFileRoute("/course/")({
  component: CoursePage,
});

// --- Main Page Component ---

function CoursePage() {
  const coursesQuery = useQuery(() => ({
    queryKey: ["courses"],
    queryFn: fetchCourses,
  }));

  return (
    <div class="bg-white min-h-full">
      <LearningBanner />
      <main class="max-w-7xl mx-auto py-12 px-4 sm:px-6 lg:px-8">
        <Suspense fallback={<p>Loading courses...</p>}>
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-x-6 gap-y-10">
            <For each={coursesQuery.data}>
              {(course) => <CourseCard course={course} />}
            </For>
          </div>
        </Suspense>
      </main>
    </div>
  );
}

// --- Sub-Component: Learning Banner ---

const LearningBanner: Component = () => {
  return (
    <div class="bg-gray-800 text-white">
      <div class="max-w-7xl mx-auto py-12 px-4 sm:px-6 lg:px-8">
        <h1 class="text-4xl font-bold tracking-tight">Learning</h1>
      </div>
    </div>
  );
};

// --- Sub-Component: Course Card ---

interface CourseCardProps {
  course: Course;
}

const CourseCard: Component<CourseCardProps> = (props) => {
  return (
    <Link
      to="/course/$courseId"
      params={{ courseId: props.course.id.toString() }}
      class="group flex flex-col"
    >
      <div class="aspect-video bg-gray-200 rounded-md overflow-hidden border border-gray-200 group-hover:shadow-lg transition-shadow">
        <img
          src={props.course.thumbnailUrl}
          alt={`Thumbnail for ${props.course.title}`}
          class="w-full h-full object-cover"
        />
      </div>
      <div class="mt-4">
        <h3 class="text-md font-bold text-gray-800 group-hover:text-blue-600 transition-colors">
          {props.course.title}
        </h3>
        <p class="mt-1 text-sm text-gray-500">{props.course.instructorName}</p>
      </div>
      <Show when={typeof props.course.progressPercent === "number"}>
        <div class="mt-2 flex-grow flex flex-col justify-end">
          <div class="w-full bg-gray-200 rounded-full h-1.5">
            <div
              class="bg-blue-600 h-1.5 rounded-full"
              style={{ width: `${props.course.progressPercent}%` }}
            />
          </div>
          <p class="mt-1 text-xs text-gray-500">
            {props.course.progressPercent}% complete
          </p>
        </div>
      </Show>
    </Link>
  );
};
