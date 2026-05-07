import type { ReactNode } from 'react';
import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

type PageToolbarProps = {
  title: string;
  description?: string;
  leading?: ReactNode;
  actions?: ReactNode;
  filters?: ReactNode;
};

export default function PageToolbar({
  title,
  description,
  leading,
  actions,
  filters,
}: PageToolbarProps) {
  return (
    <Stack spacing={2}>
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={1.75}
        justifyContent="space-between"
        alignItems={{ xs: 'flex-start', md: 'center' }}
      >
        <Stack spacing={1} sx={{ minWidth: 0 }}>
          {leading}
          <Box>
            <Typography variant="h2">{title}</Typography>
            {description ? (
              <Typography variant="body1" color="text.secondary" sx={{ mt: 0.5, maxWidth: 760 }}>
                {description}
              </Typography>
            ) : null}
          </Box>
        </Stack>
        {actions ? (
          <Stack
            direction="row"
            spacing={1}
            flexWrap="wrap"
            useFlexGap
            role="group"
            aria-label={`${title}页面操作`}
            justifyContent={{ xs: 'flex-start', md: 'flex-end' }}
            sx={{ width: { xs: '100%', md: 'auto' } }}
          >
            {actions}
          </Stack>
        ) : null}
      </Stack>
      {filters ? (
        <Box
          component="section"
          aria-label={`${title}筛选`}
          sx={{
            p: { xs: 1.25, md: 1.5 },
            borderRadius: 3,
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        >
          {filters}
        </Box>
      ) : null}
    </Stack>
  );
}
