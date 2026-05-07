import type { PropsWithChildren, ReactNode } from 'react';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Divider from '@mui/material/Divider';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

type PageCardProps = PropsWithChildren<{
  title: string;
  description?: string;
  action?: ReactNode;
  testId?: string;
}>

export default function PageCard({ title, description, action, children, testId }: PageCardProps) {
  return (
    <Card elevation={0} data-testid={testId}>
      <CardContent sx={{ p: { xs: 2, md: 2.5 } }}>
        <Stack spacing={2}>
          <Stack
            direction={{ xs: 'column', sm: 'row' }}
            spacing={1.25}
            justifyContent="space-between"
            alignItems={{ xs: 'flex-start', sm: 'center' }}
          >
            <Stack spacing={0.5}>
              <Typography variant="h4">{title}</Typography>
              {description ? (
                <Typography variant="body2" color="text.secondary">
                  {description}
                </Typography>
              ) : null}
            </Stack>
            {action ? (
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                justifyContent="flex-end"
                flexWrap="wrap"
                useFlexGap
                role="group"
                aria-label={`${title}操作`}
                sx={{ minHeight: 32 }}
              >
                {action}
              </Stack>
            ) : null}
          </Stack>
          <Divider />
          <Stack spacing={1.75}>{children}</Stack>
        </Stack>
      </CardContent>
    </Card>
  );
}
